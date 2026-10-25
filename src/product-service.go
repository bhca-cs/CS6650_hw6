package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ===================== CONFIGURATION =====================

const (
	numProducts      = 100000 // generated at startup to simulate a realistic memory footprint
	searchCheckLimit = 100    // each search checks exactly this many products, then stops
	searchMaxResults = 20     // at most this many matches are returned
	queryMaxLength   = 100    // product-search.yaml: q has maxLength 100
)

// Sample pools used to generate products. The brand doubles as the
// manufacturer, so a product's name and manufacturer stay consistent.
var brands = []string{"Alpha", "Beta", "Gamma", "Delta", "Nova", "Apex", "Zenith", "Orion"}

// Products only carry a category_id, so this slice maps ids to names for
// searching. category_id N maps to categoryNames[N-1] (ids start at 1).
var categoryNames = []string{
	"Electronics", "Books", "Home", "Sports", "Toys",
	"Beauty", "Garden", "Automotive", "Grocery", "Clothing",
}

// categoryName returns the name for a category id, or "" if the id is
// outside the known range (possible for products updated via POST).
func categoryName(id int) string {
	if id < 1 || id > len(categoryNames) {
		return ""
	}
	return categoryNames[id-1]
}

// ===================== DATA MODEL =====================

// Product mirrors the "Product" schema in api.yaml. All 6 schema fields are required.
// Name is an extra field: the search needs a product name, but the schema has none.
// OpenAPI allows additional properties by default, so this stays within the contract.
type Product struct {
	ProductID    int    `json:"product_id" binding:"required,min=1"`
	SKU          string `json:"sku" binding:"required,min=1,max=100"`
	Manufacturer string `json:"manufacturer" binding:"required,min=1,max=200"`
	CategoryID   int    `json:"category_id" binding:"required,min=1"`
	Weight       int    `json:"weight" binding:"min=0"`
	SomeOtherID  int    `json:"some_other_id" binding:"required,min=1"`
	Name         string `json:"name"`
}

// ErrorResponse mirrors the "Error" schema in api.yaml.
type ErrorResponse struct {
	Error   string `json:"error"`
	Message string `json:"message"`
	Details string `json:"details,omitempty"`
}

// SearchResponse mirrors the 200 response of GET /products/search in product-search.yaml.
type SearchResponse struct {
	Products   []Product `json:"products"`
	TotalFound int       `json:"total_found"`
	SearchTime string    `json:"search_time"`
}

// ===================== IN-MEMORY STORAGE =====================

// ProductStore holds products in a sync.Map (map[int]Product), which is safe
// for concurrent use without an explicit lock. It suits this workload: the
// products are written once at startup and then read far more than written.
type ProductStore struct {
	products sync.Map
}

func (s *ProductStore) Get(id int) (Product, bool) {
	v, ok := s.products.Load(id)
	if !ok {
		return Product{}, false
	}
	return v.(Product), true
}

func (s *ProductStore) Exists(id int) bool {
	_, ok := s.products.Load(id)
	return ok
}

func (s *ProductStore) Set(id int, p Product) {
	s.products.Store(id, p)
}

// Search checks exactly searchCheckLimit products (bounded iteration) and
// returns up to searchMaxResults whose name or category contains the query,
// case-insensitively. It counts products CHECKED, not products MATCHED, so
// every search does the same fixed amount of work regardless of the query.
// It also returns how many products were checked, so this can be verified.
func (s *ProductStore) Search(query string) ([]Product, int) {
	q := strings.ToLower(query)
	results := make([]Product, 0, searchMaxResults) // never nil, so JSON shows [] not null
	checked := 0

	s.products.Range(func(_, value any) bool {
		p := value.(Product)
		checked++
		if len(results) < searchMaxResults &&
			(strings.Contains(strings.ToLower(p.Name), q) ||
				strings.Contains(strings.ToLower(categoryName(p.CategoryID)), q)) {
			results = append(results, p)
		}
		return checked < searchCheckLimit // returning false stops Range
	})

	return results, checked
}

var store = &ProductStore{}
var errInvalidID = errors.New("productId must be a positive integer")

// seedProducts generates numProducts products at startup.
// Name format: "Product [Brand] [ID]", e.g. "Product Alpha 1".
// Categories rotate through categoryNames using modulo.
func seedProducts() {
	for i := 1; i <= numProducts; i++ {
		brand := brands[i%len(brands)]
		store.Set(i, Product{
			ProductID:    i,
			SKU:          fmt.Sprintf("SKU-%06d", i),
			Manufacturer: brand,
			CategoryID:   (i % len(categoryNames)) + 1,
			Weight:       i % 5000,
			SomeOtherID:  i,
			Name:         fmt.Sprintf("Product %s %d", brand, i),
		})
	}

	// sync.Map keeps freshly stored entries in an internal "dirty" map. The
	// first Range copies them into its read-only map once, which takes time
	// for 100,000 entries. Doing that here means the first real search
	// isn't unusually slow.
	store.products.Range(func(_, _ any) bool { return false })
}

// ===================== OBSERVABILITY: PROMETHEUS METRICS =====================

// The three RED signals (Rate, Errors, Duration) are implemented using two
// Prometheus metric types:
//
//   - httpRequestsTotal (a Counter, labeled by endpoint/method/status):
//     covers both Rate and Errors. Prometheus computes "requests per second"
//     from this ever-increasing counter using rate(...) over a time window -
//     the counter itself just counts, it doesn't store a rate directly.
//     Filtering/summing this same counter by status (e.g. status != "2xx")
//     gives the error rate - we don't need a second, separate error counter.
//
//   - httpRequestDuration (a Histogram, labeled by endpoint/method):
//     records how long each request took, sorted into buckets, so Grafana
//     can compute percentiles (p50, p95) from it - not just a single average,
//     which would hide tail latency.
var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests, labeled by endpoint, method, and status code.",
		},
		[]string{"endpoint", "method", "status"},
	)

	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "http_request_duration_seconds",
			Help: "HTTP request latency in seconds, labeled by endpoint and method.",
			// Buckets start at 0.1ms: in HW5, server-side requests took ~0.03-0.1ms,
			// so a 5ms smallest bucket made p95 always read 4.75ms. The finer low
			// buckets keep p95 meaningful at idle, and the upper buckets still
			// cover multi-second latency under stress.
			Buckets: []float64{.0001, .00025, .0005, .001, .0025, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 30},
		},
		[]string{"endpoint", "method"},
	)
)

func init() {
	// Metrics must be registered once before they can be scraped; without this,
	// promhttp.Handler() wouldn't know these two metrics exist.
	prometheus.MustRegister(httpRequestsTotal)
	prometheus.MustRegister(httpRequestDuration)
}

// metricsMiddleware wraps every incoming request. Using Gin middleware means
// this instrumentation is written once and applied to every route automatically,
// rather than repeating timing/logging code inside each individual handler.
func metricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		// c.FullPath() returns the route PATTERN (e.g. "/products/:productId"),
		// not the literal requested URL (e.g. "/products/7"). Using the pattern
		// keeps the metric's cardinality fixed regardless of how many different
		// product IDs or search queries get requested.
		endpoint := c.FullPath()
		method := c.Request.Method

		c.Next() // let the actual handler run

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		httpRequestsTotal.WithLabelValues(endpoint, method, status).Inc()
		httpRequestDuration.WithLabelValues(endpoint, method).Observe(duration)

		// ---- Structured JSON logging: the same RED data, a second way ----
		// One JSON line per request, meant to be read after the fact when
		// debugging something that happened while nobody had Grafana open.
		logEntry := map[string]interface{}{
			"timestamp":   time.Now().Format(time.RFC3339),
			"endpoint":    endpoint,
			"method":      method,
			"status_code": c.Writer.Status(),
			"duration_ms": duration * 1000,
		}
		logJSON, _ := json.Marshal(logEntry)
		log.Println(string(logJSON))
	}
}

// ===================== SETUP =====================

func main() {
	router := gin.Default()

	// Applied once here, so every route below is automatically instrumented.
	router.Use(metricsMiddleware())

	seedProducts()
	log.Printf("seeded %d products", numProducts)

	// /products/search is a static route next to the /products/:productId
	// wildcard. Recent Gin versions route the static path first.
	router.GET("/products/search", searchProducts)
	router.GET("/products/:productId", getProduct)
	router.POST("/products/:productId/details", addProductDetails)

	// Health check for the ALB target group (HW6 Part III).
	router.GET("/health", healthCheck)

	// promhttp.Handler() exposes everything registered above in Prometheus's
	// text-based scrape format. gin.WrapH adapts a standard http.Handler so it
	// can be mounted as a normal Gin route.
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	router.Run(":8080")
}

// ===================== HANDLERS =====================

// searchProducts implements GET /products/search?q={query}.
// 200 + results on success, 400 if q is missing or not 1-100 characters.
// total_found is the number of products returned (at most 20), since the
// spec gives total_found a maximum of 20.
func searchProducts(c *gin.Context) {
	query, present := c.GetQuery("q")
	length := utf8.RuneCountInString(query) // count characters, not bytes
	if !present || length < 1 || length > queryMaxLength {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "INVALID_INPUT", Message: "Invalid query parameter q", Details: "q must be between 1 and 100 characters",
		})
		return
	}

	start := time.Now()
	results, checked := store.Search(query)
	elapsed := time.Since(start)

	// Extra response header (outside the JSON body, so the spec is unchanged)
	// to verify each search checks exactly 100 products.
	c.Header("X-Products-Checked", strconv.Itoa(checked))

	c.JSON(http.StatusOK, SearchResponse{
		Products:   results,
		TotalFound: len(results),
		SearchTime: elapsed.String(), // e.g. "12.5µs": time spent in the search itself
	})
}

// healthCheck returns 200 while the service is running, so the load
// balancer can tell healthy tasks from unhealthy ones.
func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

// getProduct implements GET /products/{productId}: 200 + Product on success,
// 404 + Error if not found.
func getProduct(c *gin.Context) {
	productID, err := parseProductID(c)
	if err != nil {
		// An invalid ID isn't its own listed response code in api.yaml for GET,
		// so it's treated the same as "not found" - an invalid ID can never
		// resolve to a real product either way.
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "PRODUCT_NOT_FOUND", Message: "Product not found", Details: err.Error(),
		})
		return
	}

	product, found := store.Get(productID)
	if !found {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "PRODUCT_NOT_FOUND", Message: "Product not found", Details: "No product exists with the given ID",
		})
		return
	}

	c.JSON(http.StatusOK, product)
}

// addProductDetails implements POST /products/{productId}/details.
// Per clarification from the TA on Piazza: this endpoint is about the DETAILS,
// not the product itself - the product must already exist, or this returns 404.
// It does not create a new product.
func addProductDetails(c *gin.Context) {
	productID, err := parseProductID(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "INVALID_INPUT", Message: "Invalid product ID", Details: err.Error(),
		})
		return
	}

	if !store.Exists(productID) {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "PRODUCT_NOT_FOUND", Message: "Product not found", Details: "Cannot add details to a product that does not exist",
		})
		return
	}

	var body Product
	// ShouldBindJSON enforces the struct tags above (required, min, max) and
	// also catches malformed JSON itself.
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "INVALID_INPUT", Message: "The provided input data is invalid", Details: err.Error(),
		})
		return
	}

	// Gin's binding tags can't express this: the path's productId and the
	// body's product_id must refer to the SAME product.
	if body.ProductID != productID {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "INVALID_INPUT", Message: "product_id in request body does not match productId in URL path", Details: "The product being updated must match the URL",
		})
		return
	}

	store.Set(productID, body)
	c.Status(http.StatusNoContent)
}

// ===================== HELPERS =====================

// parseProductID extracts and validates the :productId path parameter,
// enforcing api.yaml's "integer, minimum: 1" constraint.
func parseProductID(c *gin.Context) (int, error) {
	idStr := c.Param("productId")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return 0, errInvalidID
	}
	if id < 1 {
		return 0, errInvalidID
	}
	return id, nil
}

package main

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"remittance-service/internal/boa"
	"remittance-service/internal/cybersource"
	"remittance-service/internal/database"
	"remittance-service/internal/domain"
	"remittance-service/internal/handler"
	"remittance-service/internal/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/spf13/viper"
)

func main() {
	// ─── Load Configuration ─────────────────────────────────────────────────
	cfg, err := loadConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// ─── Initialize Database (PostgreSQL) ───────────────────────────────────
	db, err := database.NewConnection(*cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	if err := db.InitializeSchema(); err != nil {
		log.Fatalf("Failed to initialize database schema: %v", err)
	}

	// ─── Initialize CyberSource Client (Inbound Collection) ─────────────────
	csRESTClient, err := cybersource.NewRESTClient(
		cfg.CyberSource.MerchantID,
		cfg.CyberSource.KeyID,
		cfg.CyberSource.SharedSecret,
		cfg.CyberSource.BaseURL,
	)
	if err != nil {
		log.Fatalf("Failed to create CyberSource REST client: %v", err)
	}

	// ─── Initialize Redis ───────────────────────────────────────────────────
	rc := domain.InitRedis("localhost", "6379")
	if rc == nil {
		log.Fatalf("Failed to initialize Redis")
	}

	// ─── Initialize Bank of Abyssinia Client (Outbound Payout) ──────────────
	var boaClient domain.BoAClient
	if cfg.BoA.MockMode {
		log.Printf("INFO: Using MOCK Bank of Abyssinia client")
		boaClient = boa.NewMockClient()
	} else {
		log.Printf("INFO: Using REAL Bank of Abyssinia client (BaseURL: %s)", cfg.BoA.BaseURL)
		var err error
		boaClient, err = boa.NewClient(
			cfg.BoA.BaseURL,
			cfg.BoA.TokenURL,
			cfg.BoA.ClientID,
			cfg.BoA.ClientSecret,
			cfg.BoA.RefreshToken,
			cfg.BoA.APIKey,
			func(newToken string) {
				// Persist the rotated refresh token back to config.yaml
				viper.Set("boa.refresh_token", newToken)
				if err := viper.WriteConfig(); err != nil {
					log.Printf("ERROR: Failed to save rotated BoA refresh token to file: %v", err)
				} else {
					log.Printf("INFO: BoA refresh token rotated and saved to config file")
				}
			},
		)
		if err != nil {
			log.Fatalf("Failed to create BoA client: %v", err)
		}
	}

	// ─── Initialize Services ────────────────────────────────────────────────
	payoutSvc := service.NewPayoutService(boaClient)

	// We use a pointer or a variable to avoid circular dependency during initialization
	var remittanceSvc domain.RemittanceService

	onCollected := func(ID uuid.UUID) {
		log.Printf("INFO: Automatic payout triggered for remittance %s", ID)
		go func() {
			_, err := remittanceSvc.ExecutePayout(ID)
			if err != nil {
				log.Printf("ERROR: Automatic payout failed for %s: %v", ID, err)
			}
		}()
	}

	collectionSvc := service.NewCollectionService(csRESTClient, rc, *db, cfg.CyberSource.ReturnURL, cfg.CyberSource.MerchantID, onCollected)
	remittanceSvc = service.NewRemittanceService(collectionSvc, payoutSvc, *db, cfg.CyberSource.TargetOrigins)

	// ─── Initialize Handlers ────────────────────────────────────────────────
	collectionHandler := handler.NewCollectionHandler(collectionSvc)
	payoutHandler := handler.NewPayoutHandler(payoutSvc)
	remittanceHandler := handler.NewRemittanceHandler(remittanceSvc)

	// ─── Setup Echo Server ──────────────────────────────────────────────────
	e := echo.New()
	e.HideBanner = true

	// === CSP Middleware (MUST BE FIRST) ===
	e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// Set the CSP header
			val := "default-src 'self' https://*.cybersource.com https://*.cardinalcommerce.com https://*.cardinaltrusted.com; " +
				"script-src 'self' 'unsafe-inline' 'unsafe-eval' blob: https://*.cybersource.com https://*.cardinalcommerce.com https://*.cardinaltrusted.com; " +
				"connect-src 'self' https://*.cybersource.com https://*.cardinalcommerce.com https://*.cardinaltrusted.com; " +
				"frame-src 'self' https://*.cybersource.com https://*.cardinalcommerce.com https://*.cardinaltrusted.com; " +
				"frame-ancestors 'self' https://*.cybersource.com https://*.cardinalcommerce.com https://*.cardinaltrusted.com; " +
				"img-src 'self' data: https://*.cybersource.com https://*.cardinalcommerce.com https://*.cardinaltrusted.com https://img.icons8.com; " +
				"style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
				"font-src 'self' https://fonts.gstatic.com;"
			c.Response().Header().Set("Content-Security-Policy", val)
			return next(c)
		}
	})

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.RequestID())
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: cfg.CORS.AllowedOrigins,
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodOptions},
		AllowHeaders: []string{
			echo.HeaderOrigin,
			echo.HeaderContentType,
			echo.HeaderAccept,
			echo.HeaderXRequestID,
			echo.HeaderAuthorization,
		},
	}))
	// add error handler middleware to catch errors and return JSON responses
	e.HTTPErrorHandler = func(err error, c echo.Context) {
		// log the error
		log.Printf("ERROR: %v", err)

		// Determine the status code
		code := http.StatusInternalServerError
		if he, ok := err.(*echo.HTTPError); ok {
			code = he.Code
		}

		// Return JSON response
		c.JSON(code, echo.Map{
			"message": err.Error(),
		})
	}

	// ─── API Routes ─────────────────────────────────────────────────────────

	api := e.Group("/api")

	// === Remittance (End-to-End Flow) ===
	// POST /api/remittance             - Initiate a remittance (validate + checkout fields)
	// POST /api/remittance/payout      - Manually trigger payout for a collected remittance
	// GET  /api/remittance/status/:id   - Get remittance status
	api.POST("/remittance", remittanceHandler.InitiateRemittance)
	api.POST("/remittance/payout", remittanceHandler.TriggerPayout)
	api.GET("/remittance/status/:id", remittanceHandler.GetStatus)
	api.GET("/remittance/sender/:email", remittanceHandler.ListSenderRemittances)
	api.GET("/remittance/receiver/:phone", remittanceHandler.ListReceiverRemittances)

	// === Collection (CyberSource Inbound) ===
	// Flex Microform & 3DS
	api.POST("/collection/capture-context", collectionHandler.CreateCaptureContext)
	api.POST("/collection/pa-setup", collectionHandler.SetupPayerAuth)
	api.POST("/collection/authorize", collectionHandler.AuthorizePayment)
	api.POST("/collection/review", collectionHandler.ReviewPayment)
	api.POST("/collection/webhook", collectionHandler.HandleWebhook)
	api.GET("/collection/saved-cards", collectionHandler.GetSenderCards)
	api.GET("/collection/config", collectionHandler.GetConfig)

	// 3DS Return (Moved to root to bypass group middleware issues)
	api.POST("/collection/return", collectionHandler.Handle3DSReturn)
	// e.Match([]string{http.MethodGet, http.MethodPost}, "/api/collection/return/", collectionHandler.Handle3DSReturn)

	// === Payout (Bank of Abyssinia Outbound) ===
	// POST /api/payout/validate      - Validate beneficiary account/wallet
	// GET  /api/payout/rate/:currency - Get exchange rate
	// GET  /api/payout/banks          - Get available banks for other-bank transfer
	// GET  /api/payout/balance        - Get settlement account balance
	// GET  /api/payout/status/:id     - Check payout remittance status
	payout := api.Group("/payout")
	payout.POST("/validate", payoutHandler.ValidateBeneficiary)
	payout.GET("/rate/:currency", payoutHandler.GetExchangeRate)
	payout.GET("/banks", payoutHandler.GetBanks)
	payout.GET("/balance", payoutHandler.GetBalance)
	payout.GET("/status/:id", payoutHandler.CheckRemittanceStatus)

	// ─── Checkout Result Pages ──────────────────────────────────────────────
	e.GET("/checkout/success", func(c echo.Context) error {
		return c.File("frontend/checkout/success.html")
	})
	e.GET("/checkout/declined", func(c echo.Context) error {
		return c.File("frontend/checkout/declined.html")
	})
	e.GET("/checkout/error", func(c echo.Context) error {
		return c.File("frontend/checkout/error.html")
	})
	e.GET("/checkout/cancelled", func(c echo.Context) error {
		return c.File("frontend/checkout/cancelled.html")
	})
	e.GET("/checkout/review", func(c echo.Context) error {
		return c.File("frontend/checkout/review.html")
	})

	// ─── Serve Frontend Static Files ────────────────────────────────────────
	e.Static("/", "frontend")

	// ─── Health Check ───────────────────────────────────────────────────────
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{
			"status":  "ok",
			"service": "remittance-service",
		})
	})

	// ─── Start Server ───────────────────────────────────────────────────────
	addr := fmt.Sprintf(":%s", cfg.Server.Port)
	log.Printf("═══════════════════════════════════════════════════════════════")
	log.Printf("  Remittance Service starting on %s", addr)
	log.Printf("  CyberSource Checkout: %s", cfg.CyberSource.CheckoutURL)
	log.Printf("  BoA Base URL:         %s", cfg.BoA.BaseURL)
	log.Printf("  CORS Allowed Origins: %s", strings.Join(cfg.CORS.AllowedOrigins, ", "))
	log.Printf("═══════════════════════════════════════════════════════════════")

	if err := e.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

func loadConfig() (*domain.Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./config")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg domain.Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

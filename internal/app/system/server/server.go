// internal/app/system/server/server.go
package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dalemusser/stratasave/internal/app/system/config"
	"github.com/dalemusser/stratasave/internal/app/system/handler"
	"github.com/dalemusser/stratasave/internal/app/system/routes"

	"github.com/andybalholm/brotli"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/crypto/acme/autocert"
)

/* -------------------------------------------------------------------------- */
/* PUBLIC ENTRY                                                               */
/* -------------------------------------------------------------------------- */

// StartServerWithContext launches the HTTP/HTTPS server and blocks until the
// provided context is canceled. It logs an explicit “listening” message after
// binding sockets so you know the server actually accepted connections.
func StartServerWithContext(ctx context.Context, cfg *config.Config, mongoClient *mongo.Client) error {
	/* ---------- router & middleware --------------------------------------- */
	h := handler.NewHandler(cfg, mongoClient)
	r := chi.NewRouter()

	// Early drop of obvious junk (no logs for these)
	r.Use(blockScans) // short-circuit scanner junk

	// Request context & safety
	r.Use(middleware.RequestID) // stable ID for tracing
	r.Use(middleware.RealIP)    // correct client IP
	r.Use(middleware.Recoverer) // turn panics into 500s

	// Cross-cutting behavior that affects responses

	// CORS (from config)
	if cfg.EnableCORS {
		r.Use(corsMiddleware(cfg)) // add CORS headers
		zap.L().Info("CORS enabled",
			zap.Int("origins", len(cfg.CORSAllowedOrigins)),
			zap.Int("methods", len(cfg.CORSAllowedMethods)),
			zap.Int("headers", len(cfg.CORSAllowedHeaders)),
			zap.Int("exposed_headers", len(cfg.CORSExposedHeaders)),
			zap.Bool("allow_credentials", cfg.CORSAllowCredentials),
			zap.Int("max_age", cfg.CORSMaxAge),
		)
	}

	// Compression
	if cfg.EnableCompression {
		// Use a pluggable compressor so we can offer Brotli ("br") + gzip fallback.
		// Level semantics:
		//   - gzip: stdlib levels 0..9 (5 is a good balance)
		//   - br:   quality 0..11 (we clamp below)
		comp := middleware.NewCompressor(5) // base level for gzip
		comp.SetEncoder("br", func(w io.Writer, level int) io.Writer {
			// Clamp to Brotli's 0..11 range.
			if level < 0 {
				level = 0
			} else if level > 11 {
				level = 11
			}
			return brotli.NewWriterOptions(w, brotli.WriterOptions{Quality: level})
		})
		r.Use(comp.Handler)
		zap.L().Info("response compression enabled", zap.String("encodings", "br,gzip"))
	}

	// Access logging (must be last to see final status/bytes/latency)
	r.Use(zapRequestLogger(zap.L())) // structured access logger

	// Register routes
	routes.RegisterAllRoutes(r, h)

	/* ---------- build http.Server with sane timeouts ---------------------- */
	srv := &http.Server{
		Handler:           r,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		// TLSConfig set below for HTTPS modes.
	}
	// Route stdlib server errors into Zap (use Warn to avoid chatty info)
	if l, err := zap.NewStdLogAt(zap.L(), zapcore.WarnLevel); err != nil {
		zap.L().Warn("failed to attach stdlib error logger", zap.Error(err))
	} else {
		srv.ErrorLog = l
	}

	httpAddr := ":" + strconv.Itoa(cfg.HTTPPort)
	httpsAddr := ":" + strconv.Itoa(cfg.HTTPSPort)

	var (
		auxSrv   *http.Server // :80 ACME/redirect server (when HTTPS/http-01)
		ln       net.Listener // primary listener we Serve() on
		serveErr = make(chan error, 1)
		auxErr   chan error // lazily created if auxSrv is started
		err      error
	)

	/* ---------- select serving mode --------------------------------------- */
	switch {
	// ----------------------------- HTTP only -------------------------------
	case !cfg.UseHTTPS:
		ln, err = net.Listen("tcp", httpAddr)
		if err != nil {
			return fmt.Errorf("listen http %s: %w", httpAddr, err)
		}
		zap.L().Info("HTTP server listening", zap.String("addr", ln.Addr().String()))
		go servePrimary(srv, ln, serveErr)

	// ----------------------- HTTPS via Let's Encrypt (http-01) ------------
	case cfg.UseLetsEncrypt && strings.ToLower(strings.TrimSpace(cfg.LetsEncryptChallenge)) == "http-01":
		m := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			HostPolicy: autocert.HostWhitelist(cfg.Domain),
			Cache:      autocert.DirCache(cfg.LetsEncryptCacheDir),
			Email:      cfg.LetsEncryptEmail,
		}

		// Port 80: ACME challenge + HTTPS redirect for everything else.
		auxSrv = &http.Server{
			Addr:              ":80",
			Handler:           m.HTTPHandler(httpRedirectHandler()),
			ReadTimeout:       15 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		if l, err := zap.NewStdLogAt(zap.L(), zapcore.WarnLevel); err == nil {
			auxSrv.ErrorLog = l
		}
		auxErr = make(chan error, 1)
		go serveAuxiliary(auxSrv, auxErr)
		zap.L().Info("ACME + redirect server listening", zap.String("addr", auxSrv.Addr))

		// pre-warm before binding :443
		if err := waitForCert(ctx, m, cfg.Domain, 60*time.Second); err != nil {
			zap.L().Warn("autocert pre-warm failed; first HTTPS hits may see TLS errors", zap.Error(err))
		}

		// Port 443: primary HTTPS.
		tlsCfg := &tls.Config{
			MinVersion:     tls.VersionTLS12,
			GetCertificate: m.GetCertificate,
		}
		srv.TLSConfig = tlsCfg

		base, e := net.Listen("tcp", httpsAddr)
		if e != nil {
			return fmt.Errorf("listen https %s: %w", httpsAddr, e)
		}
		ln = tls.NewListener(base, tlsCfg)
		zap.L().Info("HTTPS server (Let's Encrypt http-01) listening",
			zap.String("addr", httpsAddr),
			zap.String("domain", cfg.Domain))
		go servePrimary(srv, ln, serveErr)

	// ----------------------- HTTPS via manual certs ------------------------
	default:
		if cfg.CertFile == "" || cfg.KeyFile == "" {
			return fmt.Errorf("manual TLS selected but cert_file / key_file not provided")
		}

		// Port 80: redirect everything to HTTPS.
		auxSrv = &http.Server{
			Addr:              ":80",
			Handler:           httpRedirectHandler(),
			ReadTimeout:       15 * time.Second,
			ReadHeaderTimeout: 10 * time.Second,
			WriteTimeout:      60 * time.Second,
			IdleTimeout:       120 * time.Second,
		}
		if l, err := zap.NewStdLogAt(zap.L(), zapcore.WarnLevel); err == nil {
			auxSrv.ErrorLog = l
		}
		auxErr = make(chan error, 1)
		go serveAuxiliary(auxSrv, auxErr)
		zap.L().Info("HTTP → HTTPS redirect server listening", zap.String("addr", auxSrv.Addr))

		// Port 443: primary HTTPS with provided certs.
		cert, e := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFile)
		if e != nil {
			return fmt.Errorf("load TLS cert/key: %w", e)
		}
		tlsCfg := &tls.Config{
			MinVersion:   tls.VersionTLS12,
			Certificates: []tls.Certificate{cert},
		}
		srv.TLSConfig = tlsCfg

		base, e := net.Listen("tcp", httpsAddr)
		if e != nil {
			return fmt.Errorf("listen https %s: %w", httpsAddr, e)
		}
		ln = tls.NewListener(base, tlsCfg)
		zap.L().Info("HTTPS server (manual TLS) listening",
			zap.String("addr", httpsAddr),
			zap.String("cert_file", cfg.CertFile))
		go servePrimary(srv, ln, serveErr)
	}

	/* ---------- wait for shutdown / errors -------------------------------- */
	for {
		select {
		case <-ctx.Done():
			// Graceful shutdown path requested by caller.
			zap.L().Info("shutting down server…")
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			_ = shutdownAux(auxSrv, shutdownCtx)
			if err := srv.Shutdown(shutdownCtx); err != nil {
				return fmt.Errorf("server shutdown: %w", err)
			}
			zap.L().Info("server stopped gracefully")
			return nil

		case err := <-serveErr:
			// Primary server crashed or closed unexpectedly.
			if err != nil {
				return fmt.Errorf("primary server error: %w", err)
			}
			// nil means it was closed cleanly (rare here). Ensure aux is stopped too.
			_ = shutdownAux(auxSrv, context.Background())
			return nil

		case err := <-auxErr:
			// Auxiliary server (ACME / redirect) crashed.
			if err != nil {
				// Stop primary as well; ACME/redirect failure in HTTPS modes is fatal.
				_ = srv.Close()
				return fmt.Errorf("auxiliary server error: %w", err)
			}
			// nil (ErrServerClosed) → continue waiting for ctx or primary.
			auxSrv = nil
			auxErr = nil
		}
	}
}

/* -------------------------------------------------------------------------- */
/* HELPERS                                                                    */
/* -------------------------------------------------------------------------- */

// corsMiddleware builds a CORS handler from config (no hard-coded values).
func corsMiddleware(cfg *config.Config) func(http.Handler) http.Handler {
	opts := cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   cfg.CORSAllowedMethods,
		AllowedHeaders:   cfg.CORSAllowedHeaders,
		ExposedHeaders:   cfg.CORSExposedHeaders,
		AllowCredentials: cfg.CORSAllowCredentials,
		MaxAge:           cfg.CORSMaxAge,
	}
	return cors.Handler(opts)
}

// httpRedirectHandler redirects any HTTP request to HTTPS preserving host + path.
func httpRedirectHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		target := "https://" + r.Host + r.URL.RequestURI()
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	})
}

// servePrimary runs srv.Serve on the provided listener and reports terminal errors.
func servePrimary(srv *http.Server, ln net.Listener, ch chan<- error) {
	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		ch <- err
		return
	}
	ch <- nil
}

// serveAuxiliary runs auxSrv.ListenAndServe and reports terminal errors.
func serveAuxiliary(auxSrv *http.Server, ch chan<- error) {
	if err := auxSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		ch <- err
		return
	}
	ch <- nil
}

func shutdownAux(auxSrv *http.Server, ctx context.Context) error {
	if auxSrv == nil {
		return nil
	}
	return auxSrv.Shutdown(ctx)
}

// waitForCert blocks until autocert has a certificate for host (or times out).
func waitForCert(ctx context.Context, m *autocert.Manager, host string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		// Respect shutdown
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		_, err := m.GetCertificate(&tls.ClientHelloInfo{ServerName: host})
		if err == nil {
			return nil // cert is ready and cached
		}
		lastErr = err

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for cert for %q: %w", host, lastErr)
		}
		time.Sleep(1 * time.Second)
	}
}

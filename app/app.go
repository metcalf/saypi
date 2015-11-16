package app

import (
	"io"
	"net/http"

	"gopkg.in/throttled/throttled.v2"
	"gopkg.in/throttled/throttled.v2/store/memstore"

	"github.com/codahale/http-handlers/recovery"
	"github.com/jmoiron/sqlx"
	"github.com/metcalf/saypi/auth"
	"github.com/metcalf/saypi/dbutil"
	"github.com/metcalf/saypi/log"
	"github.com/metcalf/saypi/mux"
	"github.com/metcalf/saypi/say"
)

// Configuration represents the configuration for an App
type Configuration struct {
	DBDSN     string // postgres data source name
	DBMaxIdle int    // maximum number of idle DB connections
	DBMaxOpen int    // maximum number of open DB connections

	IPPerMinute int // maximum number of requests per IP per minute
	IPRateBurst int // maximum burst of requests from an IP

	UserSecret []byte // secret for generating secure user tokens
}

// App encapsulates the handlers for the saypi API
type App struct {
	Srv     http.Handler
	closers []io.Closer
}

// Close cleans up any resources used by the app such as database connections.
func (a *App) Close() error {
	return closeAll(a.closers)
}

// New creates an App for the given configuration.
func New(config *Configuration) (*App, error) {
	var app App

	db, err := buildDB(config.DBDSN, config.DBMaxIdle, config.DBMaxOpen)
	if err != nil {
		defer app.Close()
		return nil, err
	}
	app.closers = append(app.closers, db)

	ipQuota := throttled.RateQuota{throttled.PerMin(config.IPPerMinute), config.IPRateBurst}
	ipLimiter, err := buildLimiter(ipQuota)

	authCtrl, err := auth.New(config.UserSecret)
	if err != nil {
		defer app.Close()
		return nil, err
	}

	sayCtrl, err := say.New(db)
	if err != nil {
		defer app.Close()
		return nil, err
	}
	app.closers = append(app.closers, sayCtrl)

	mainMux := mux.New()
	mainMux.RouteFuncC(mux.Pattern("POST", "/users"), authCtrl.CreateUser)
	mainMux.RouteFuncC(mux.Pattern("GET", "/users/:id"), authCtrl.GetUser)

	privMux := mux.New()
	// TODO: We don't really want random paths returning a 403 instead of a
	mainMux.NotFoundHandler = authCtrl.WrapC(privMux)

	privMux.RouteFunc(mux.Pattern("GET", "/animals"), sayCtrl.GetAnimals)

	privMux.RouteFuncC(mux.Pattern("GET", "/moods"), sayCtrl.ListMoods)
	privMux.RouteFuncC(mux.Pattern("PUT", "/moods/:mood"), sayCtrl.SetMood)
	privMux.RouteFuncC(mux.Pattern("GET", "/moods/:mood"), sayCtrl.GetMood)
	privMux.RouteFuncC(mux.Pattern("DELETE", "/moods/:mood"), sayCtrl.DeleteMood)

	privMux.RouteFuncC(mux.Pattern("GET", "/conversations"), sayCtrl.ListConversations)
	privMux.RouteFuncC(mux.Pattern("POST", "/conversations"), sayCtrl.CreateConversation)
	privMux.RouteFuncC(mux.Pattern("GET", "/conversations/:conversation"), sayCtrl.GetConversation)
	privMux.RouteFuncC(mux.Pattern("DELETE", "/conversations/:conversation"), sayCtrl.DeleteConversation)

	privMux.RouteFuncC(mux.Pattern("POST", "/conversations/:conversation/lines"), sayCtrl.CreateLine)
	privMux.RouteFuncC(mux.Pattern("GET", "/conversations/:conversation/lines/:line"), sayCtrl.GetLine)
	privMux.RouteFuncC(mux.Pattern("DELETE", "/conversations/:conversation/lines/:line"), sayCtrl.DeleteLine)

	mw := mux.NewMiddleware()
	mw.Add(func(h http.Handler) http.Handler {
		return recovery.Wrap(h, recovery.LogOnPanic)
	})
	mw.AddC(log.WrapC)
	mw.Add(ipLimiter.RateLimit)

	app.Srv = mw.Wrap(mainMux)

	return &app, nil
}

// NewForTest creates a new App instance specifically for use in
// testing. This will modify your passed Configuration to incorporate
// testing default values. For non-stub configurations, this will
// initialize a new database and store the DSN in the Configuration.
func NewForTest(config *Configuration) (*App, error) {
	var closers []io.Closer

	if len(config.UserSecret) == 0 {
		config.UserSecret = auth.TestSecret
	}
	if config.IPPerMinute == 0 {
		config.IPPerMinute = 100000
	}
	if config.IPRateBurst == 0 {
		config.IPRateBurst = 100000
	}

	if config.DBDSN == "" {
		tdb, db, err := dbutil.NewTestDB()
		if err != nil {
			return nil, err
		}
		// We don't need the db handle
		if err := db.Close(); err != nil {
			return nil, err
		}
		closers = append(closers, tdb)

		config.DBDSN = dbutil.DefaultDataSource + " dbname=" + tdb.Name()
	}

	a, err := New(config)
	if err != nil {
		closeAll(closers)
		return nil, err
	}

	for _, closer := range closers {
		a.closers = append(a.closers, closer)
	}

	return a, nil
}

func buildDB(dsn string, maxIdle, maxOpen int) (*sqlx.DB, error) {
	db, err := sqlx.Open("postgres", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxIdleConns(maxIdle)
	db.SetMaxOpenConns(maxOpen)
	db.MapperFunc(dbutil.MapperFunc())
	return db, nil
}

func buildLimiter(quota throttled.RateQuota) (*throttled.HTTPRateLimiter, error) {
	store, err := memstore.New(65536)
	if err != nil {
		return nil, err
	}

	rateLimiter, err := throttled.NewGCRARateLimiter(store, quota)
	if err != nil {
		return nil, err
	}

	return &throttled.HTTPRateLimiter{
		RateLimiter: rateLimiter,
		VaryBy:      &throttled.VaryBy{RemoteAddr: true},
	}, nil
}

func closeAll(closers []io.Closer) error {
	for _, cls := range closers {
		if err := cls.Close(); err != nil {
			return err
		}
	}
	return nil
}

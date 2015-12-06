package app

import (
	"io"
	"net/http"

	"goji.io"
	"goji.io/pat"

	"gopkg.in/throttled/throttled.v2"
	"gopkg.in/throttled/throttled.v2/store/memstore"

	"github.com/jmoiron/sqlx"
	"github.com/metcalf/saypi/auth"
	"github.com/metcalf/saypi/dbutil"
	"github.com/metcalf/saypi/metrics"
	"github.com/metcalf/saypi/reqlog"
	"github.com/metcalf/saypi/respond"
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

var Routes = struct {
	CreateUser, GetUser,
	GetAnimals,
	ListMoods, SetMood, GetMood, DeleteMood,
	ListConversations, CreateConversation, GetConversation, DeleteConversation,
	CreateLine, GetLine, DeleteLine *pat.Pattern
}{
	CreateUser: pat.Post("/users"),
	GetUser:    pat.Get("/users/:id"),

	GetAnimals: pat.Get("/animals"),

	ListMoods:  pat.Get("/moods"),
	SetMood:    pat.Put("/moods/:mood"),
	GetMood:    pat.Get("/moods/:mood"),
	DeleteMood: pat.Delete("/moods/:mood"),

	ListConversations:  pat.Get("/conversations"),
	CreateConversation: pat.Post("/conversations"),
	GetConversation:    pat.Get("/conversations/:conversation"),
	DeleteConversation: pat.Delete("/conversations/:conversation"),

	CreateLine: pat.Post("/conversations/:conversation/lines"),
	GetLine:    pat.Get("/conversations/:conversation/lines/:line"),
	DeleteLine: pat.Delete("/conversations/:conversation/lines/:line"),
}

// App encapsulates the handlers for the saypi API
type App struct {
	srv     http.Handler
	closers []io.Closer
}

// Close cleans up any resources used by the app such as database connections.
func (a *App) Close() error {
	return closeAll(a.closers)
}

func (a *App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.srv.ServeHTTP(w, r)
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

	// TODO: Proper not found handler
	privMux := goji.NewMux()
	privMux.UseC(metrics.WrapSubmuxC)
	privMux.UseC(authCtrl.WrapC)

	privMux.HandleFuncC(Routes.GetAnimals, sayCtrl.GetAnimals)

	privMux.HandleFuncC(Routes.ListMoods, sayCtrl.ListMoods)
	privMux.HandleFuncC(Routes.SetMood, sayCtrl.SetMood)
	privMux.HandleFuncC(Routes.GetMood, sayCtrl.GetMood)
	privMux.HandleFuncC(Routes.DeleteMood, sayCtrl.DeleteMood)

	privMux.HandleFuncC(Routes.ListConversations, sayCtrl.ListConversations)
	privMux.HandleFuncC(Routes.CreateConversation, sayCtrl.CreateConversation)
	privMux.HandleFuncC(Routes.GetConversation, sayCtrl.GetConversation)
	privMux.HandleFuncC(Routes.DeleteConversation, sayCtrl.DeleteConversation)

	privMux.HandleFuncC(Routes.CreateLine, sayCtrl.CreateLine)
	privMux.HandleFuncC(Routes.GetLine, sayCtrl.GetLine)
	privMux.HandleFuncC(Routes.DeleteLine, sayCtrl.DeleteLine)

	mainMux := goji.NewMux()
	mainMux.HandleFuncC(Routes.CreateUser, authCtrl.CreateUser)
	mainMux.HandleFuncC(Routes.GetUser, authCtrl.GetUser)
	mainMux.HandleC(pat.New("/*"), privMux)

	mainMux.UseC(reqlog.WrapC)
	mainMux.UseC(respond.WrapPanicC)
	mainMux.UseC(metrics.WrapC)
	mainMux.Use(ipLimiter.RateLimit)

	app.srv = mainMux

	return &app, nil
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

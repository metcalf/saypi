package client

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"

	"github.com/metcalf/saypi/app"
	"github.com/metcalf/saypi/apptest"
	"github.com/metcalf/saypi/dbutil"
)

type TestClient struct {
	Client
	closers []io.Closer
}

func (c *TestClient) Close() error {
	for _, cls := range c.closers {
		if err := cls.Close(); err != nil {
			return err
		}
	}
	return nil
}

// NewTestClient initializes a TestClient instance with an embedded
// copy of the app. This will modify your passed Configuration to
// incorporate testing default values. For non-stub configurations,
// this will initialize a new database and store the DSN in the
// Configuration.
func NewTestClient(cfg *app.Configuration) (*TestClient, error) {
	var cli TestClient

	base := url.URL{}
	cli.baseURL = &base

	if cfg == nil {
		cfg = &app.Configuration{}
	}

	if len(cfg.UserSecret) == 0 {
		cfg.UserSecret = apptest.TestSecret
	}
	if cfg.IPPerMinute == 0 {
		cfg.IPPerMinute = 100000
	}
	if cfg.IPRateBurst == 0 {
		cfg.IPRateBurst = 100000
	}

	if cfg.DBDSN == "" {
		tdb, db, err := dbutil.NewTestDB()
		if err != nil {
			return nil, err
		}
		// We don't need the db handle
		if err := db.Close(); err != nil {
			return nil, err
		}
		cli.closers = append(cli.closers, tdb)

		cfg.DBDSN = dbutil.DefaultDataSource + " dbname=" + tdb.Name()
	}

	a, err := app.New(cfg)
	if err != nil {
		cli.Close()
		return nil, err
	}
	cli.closers = append(cli.closers, a)

	cli.do = func(req *http.Request) (*http.Response, error) {
		rr := httptest.NewRecorder()
		a.ServeHTTP(rr, req)

		resp := http.Response{
			Status:        fmt.Sprintf("%d %s", rr.Code, http.StatusText(rr.Code)),
			StatusCode:    rr.Code,
			Body:          ioutil.NopCloser(rr.Body),
			Header:        rr.HeaderMap,
			ContentLength: int64(rr.Body.Len()),
			Request:       req,
		}

		return &resp, nil
	}

	return &cli, nil
}

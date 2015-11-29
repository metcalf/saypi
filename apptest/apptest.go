package apptest

import (
	"fmt"
	"net/http/httptest"
)

const (
	// TestValidUser is a valid authentication string for the TestSecret.
	TestValidUser = "DUtRr7IlHC-fd2wH8tfX_iLM8p8-3yeF4MTbc89B1lt41mk17sOlb6sg3JF_z6Sv"
	// TestInvalidUser is an invalid authentication string for the TestSecret
	// that is the same length as a valid string so it parses the same way.
	TestInvalidUser = "ABCDr7IlHC-fd2wH8tfX_iLM8p8-3yeF4MTbc89B1lt41mk17sOlb6sg3JF_z6Sv"
)

// TestSecret is an arbitrary value for consistent testing of authentication.
var TestSecret = []byte("shhh")

// AssertStatus returns a helpful error if rr.Code != want
func AssertStatus(rr *httptest.ResponseRecorder, want int) error {
	if want == rr.Code {
		return nil
	}
	return fmt.Errorf("Expected status %d but got %d with body %s", want, rr.Code, rr.Body)
}

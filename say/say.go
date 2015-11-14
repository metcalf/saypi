package say

import "net/http"

type Controller struct {
}

func New() *Controller {
	return &Controller{}
}

func (c *Controller) GetAnimals(w http.ResponseWriter, r *http.Request) {
}

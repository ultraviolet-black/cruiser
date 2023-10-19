package server

import "net/http"

type swapHandler struct {
	handlerCh chan http.Handler
	handler   http.Handler
}

func (h *swapHandler) watchSwap() {
	for handler := range h.handlerCh {
		h.handler = handler
	}
}

func (h *swapHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.handler.ServeHTTP(w, r)
}

func (h *swapHandler) Swap(handler http.Handler) {
	h.handlerCh <- handler
}

func (h *swapHandler) Close() {
	close(h.handlerCh)
}

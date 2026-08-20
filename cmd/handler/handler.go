package handler

import "github.com/Aergiaaa/noted/service"

type Handler struct {
	Service *service.Services
}

func New(svc *service.Services) *Handler {
	return &Handler{Service: svc}
}

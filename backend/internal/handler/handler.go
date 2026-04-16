package handler

import "github.com/freewhen/backend/internal/service"

type Handler struct {
	service *service.Service
}

func New(s *service.Service) *Handler {
	return &Handler{service: s}
}

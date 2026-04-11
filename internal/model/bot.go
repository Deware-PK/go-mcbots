package model

type LaunchRequest struct {
	ID   string `json:"id" binding:"required"`
	Name string `json:"name" binding:"required"`
	Addr string `json:"addr" binding:"required"`
}

type ChatRequest struct {
	Message string `json:"message" binding:"required"`
}

type GoToRequest struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Z      float64 `json:"z"`
	Sprint bool    `json:"sprint"`
}

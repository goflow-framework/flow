package controllers

import (
	flow "github.com/undiegomejia/flow/pkg/flow"
)

// UsersController demonstrates basic controller actions for the example app.
type UsersController struct{ *flow.Controller }

func NewUsersController(app *flow.App) *UsersController {
	return &UsersController{Controller: flow.NewController(app)}
}

func (u *UsersController) Index(ctx *flow.Context) {
	// simple demo data
	data := map[string]interface{}{"Title": "Users", "Items": []string{"Alice", "Bob"}}
	_ = u.Render(ctx, "users/index", data)
}

func (u *UsersController) Show(ctx *flow.Context) {
	id := ctx.Param("id")
	data := map[string]interface{}{"Title": "User", "ID": id}
	_ = u.Render(ctx, "users/show", data)
}

func (u *UsersController) New(ctx *flow.Context) {
	ctx.JSON(200, map[string]string{"action": "new"})
}

func (u *UsersController) Create(ctx *flow.Context) {
	ctx.JSON(201, map[string]string{"action": "create"})
}

func (u *UsersController) Edit(ctx *flow.Context) {
	ctx.JSON(200, map[string]string{"action": "edit"})
}

func (u *UsersController) Update(ctx *flow.Context) {
	ctx.JSON(200, map[string]string{"action": "update"})
}

func (u *UsersController) Destroy(ctx *flow.Context) {
	ctx.JSON(200, map[string]string{"action": "destroy"})
}

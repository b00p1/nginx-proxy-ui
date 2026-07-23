package handler

import (
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

type Render struct {
	loginTmpl  *template.Template
	cpTmpl     *template.Template
	dashTmpl   *template.Template
	usersTmpl  *template.Template
}

func NewRender(fsys fs.FS) (*Render, error) {
	funcMap := template.FuncMap{
		"json": func(v interface{}) template.JS {
			b, err := json.Marshal(v)
			if err != nil {
				return template.JS("null")
			}
			return template.JS(b)
		},
		"upper": strings.ToUpper,
	}

	layout, err := fs.ReadFile(fsys, "layout.html")
	if err != nil {
		return nil, err
	}
	login, err := fs.ReadFile(fsys, "login.html")
	if err != nil {
		return nil, err
	}
	cp, err := fs.ReadFile(fsys, "change-password.html")
	if err != nil {
		return nil, err
	}
	dash, err := fs.ReadFile(fsys, "dashboard.html")
	if err != nil {
		return nil, err
	}
	users, err := fs.ReadFile(fsys, "users.html")
	if err != nil {
		return nil, err
	}

	loginTmpl, err := template.New("login").Funcs(funcMap).Parse(string(layout) + string(login))
	if err != nil {
		return nil, err
	}
	cpTmpl, err := template.New("change-password").Funcs(funcMap).Parse(string(layout) + string(cp))
	if err != nil {
		return nil, err
	}
	dashTmpl, err := template.New("dashboard").Funcs(funcMap).Parse(string(layout) + string(dash))
	if err != nil {
		return nil, err
	}
	usersTmpl, err := template.New("users").Funcs(funcMap).Parse(string(layout) + string(users))
	if err != nil {
		return nil, err
	}

	return &Render{
		loginTmpl: loginTmpl,
		cpTmpl:    cpTmpl,
		dashTmpl:  dashTmpl,
		usersTmpl: usersTmpl,
	}, nil
}

func (r *Render) Template(name string) *template.Template {
	switch name {
	case "login":
		return r.loginTmpl
	case "change-password":
		return r.cpTmpl
	case "dashboard":
		return r.dashTmpl
	case "users":
		return r.usersTmpl
	default:
		return r.dashTmpl
	}
}

func (r *Render) Users(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.usersTmpl.Execute(w, data)
}

func (r *Render) Login(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.loginTmpl.Execute(w, data)
}

func (r *Render) ChangePassword(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.cpTmpl.Execute(w, data)
}

func (r *Render) Dashboard(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	r.dashTmpl.Execute(w, data)
}

func (r *Render) JSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

package httpui

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"mycis/internal/db"
	"mycis/internal/service"

	"github.com/labstack/echo/v5"
)

const (
	createUserNameSessionKey  = "create_user_name"
	createUserEmailSessionKey = "create_user_email"
	createUserRoleSessionKey  = "create_user_role"
	createUserOpenSessionKey  = "create_user_open"
	editUserIDSessionKey      = "edit_user_id"
	editUserNameSessionKey    = "edit_user_name"
	editUserEmailSessionKey   = "edit_user_email"
	editUserRoleSessionKey    = "edit_user_role"
	editUserOpenSessionKey    = "edit_user_open"
)

func (s *Server) usersPage(c *echo.Context) error {
	if !s.requireUserManager(c) {
		return nil
	}

	users, err := s.services.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}
	createUserForm, createUserDialogOpen := s.popCreateUserFormState(c)
	if createUserForm.Role == "" {
		createUserForm.Role = string(db.UserRoleEditor)
	}
	editUserForm, editUserDialogOpen := s.popEditUserFormState(c)

	return s.render(c, "users", UsersPageData{
		BaseData:             s.baseData(c, "Users", "users"),
		Users:                users,
		Roles:                db.AllUserRoles(),
		CreateUserDialogOpen: createUserDialogOpen,
		CreateUserForm:       createUserForm,
		EditUserDialogOpen:   editUserDialogOpen,
		EditUserForm:         editUserForm,
	})
}

func (s *Server) userCreatePost(c *echo.Context) error {
	if !s.requireUserManager(c) {
		return nil
	}

	form, err := s.readFormOrRedirect(c, "/admin/users", "Could not read the user form.")
	if err != nil {
		return err
	}

	role, err := db.ParseUserRole(form.Get("role"))
	if err != nil {
		return s.redirectUserCreateFailure(c, form, err.Error())
	}

	user, password, err := s.services.Auth.CreateUser(c.Request().Context(), form.Get("name"), form.Get("email"), role)
	if err != nil {
		return s.redirectUserCreateFailure(c, form, err.Error())
	}
	return s.redirectWithFlash(c, "/admin/users", "success", fmt.Sprintf("Created %s. Temporary password: %s", user.Email, password))
}

func (s *Server) userUpdatePost(c *echo.Context) error {
	if !s.requireUserManager(c) {
		return nil
	}

	form, err := s.readFormOrRedirect(c, "/admin/users", "Could not read the user form.")
	if err != nil {
		return err
	}

	role, err := db.ParseUserRole(form.Get("role"))
	if err != nil {
		return s.redirectUserUpdateFailure(c, form, err.Error())
	}

	user, err := s.services.Auth.UpdateUser(c.Request().Context(), service.UpdateUserInput{
		ID:       form.Get("user_id"),
		Name:     form.Get("name"),
		Role:     role,
		Password: form.Get("password"),
	})
	if err != nil {
		return s.redirectUserUpdateFailure(c, form, err.Error())
	}

	message := fmt.Sprintf("Updated %s.", user.Email)
	if strings.TrimSpace(form.Get("password")) != "" {
		message = fmt.Sprintf("Updated %s and reset the password.", user.Email)
	}
	return s.redirectWithFlash(c, "/admin/users", "success", message)
}

func (s *Server) redirectUserCreateFailure(c *echo.Context, form url.Values, message string) error {
	session, err := s.session(c)
	if err == nil {
		session.Values[createUserNameSessionKey] = strings.TrimSpace(form.Get("name"))
		session.Values[createUserEmailSessionKey] = strings.TrimSpace(form.Get("email"))

		role := strings.TrimSpace(form.Get("role"))
		if role == "" {
			role = string(db.UserRoleEditor)
		}
		session.Values[createUserRoleSessionKey] = role
		session.Values[createUserOpenSessionKey] = "1"
		session.AddFlash("error|" + message)
		_ = session.Save(c.Request(), c.Response())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/users")
}

func (s *Server) popCreateUserFormState(c *echo.Context) (UserCreateFormData, bool) {
	open := s.popSessionString(c, createUserOpenSessionKey) == "1"

	return UserCreateFormData{
		Name:  s.popSessionString(c, createUserNameSessionKey),
		Email: s.popSessionString(c, createUserEmailSessionKey),
		Role:  s.popSessionString(c, createUserRoleSessionKey),
	}, open
}

func (s *Server) redirectUserUpdateFailure(c *echo.Context, form url.Values, message string) error {
	session, err := s.session(c)
	if err == nil {
		session.Values[editUserIDSessionKey] = strings.TrimSpace(form.Get("user_id"))
		session.Values[editUserNameSessionKey] = strings.TrimSpace(form.Get("name"))
		session.Values[editUserEmailSessionKey] = strings.TrimSpace(form.Get("email"))

		role := strings.TrimSpace(form.Get("role"))
		if role == "" {
			role = string(db.UserRoleEditor)
		}
		session.Values[editUserRoleSessionKey] = role
		session.Values[editUserOpenSessionKey] = "1"
		session.AddFlash("error|" + message)
		_ = session.Save(c.Request(), c.Response())
	}
	return c.Redirect(http.StatusSeeOther, "/admin/users")
}

func (s *Server) popEditUserFormState(c *echo.Context) (UserEditFormData, bool) {
	open := s.popSessionString(c, editUserOpenSessionKey) == "1"

	return UserEditFormData{
		ID:    s.popSessionString(c, editUserIDSessionKey),
		Name:  s.popSessionString(c, editUserNameSessionKey),
		Email: s.popSessionString(c, editUserEmailSessionKey),
		Role:  s.popSessionString(c, editUserRoleSessionKey),
	}, open
}

package httpui

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"

	"mycis/internal/service"
	"mycis/internal/textutil"
)

func (s *Server) itemDetailPage(c *echo.Context) error {
	itemID := c.Param("itemID")
	detail, err := s.services.Items.GetDetail(c.Request().Context(), itemID)
	if err != nil {
		return renderServiceError(c, err)
	}
	users, err := s.services.Auth.ListUsers(c.Request().Context())
	if err != nil {
		return c.String(http.StatusInternalServerError, err.Error())
	}

	return s.render(c, "item_show", ItemDetailPageData{
		BaseData: s.baseData(c, detail.Item.ItemCode+" · "+detail.Item.ItemTitle, "assessments"),
		Detail:   detail,
		Users:    users,
	})
}

func (s *Server) itemUpdatePost(c *echo.Context) error {
	itemID := c.Param("itemID")
	form, err := s.readFormOrRedirect(c, "/items/"+itemID, "Could not read the item form.")
	if err != nil {
		return err
	}

	id, err := uuid.Parse(itemID)
	if err != nil {
		return c.NoContent(http.StatusNotFound)
	}

	user := s.currentUser(c)
	input, err := s.readItemUpdateInput(form, id, user != nil && user.CanManageAssessments())
	if err != nil {
		return s.redirectWithFlash(c, "/items/"+itemID, "error", err.Error())
	}

	if err := s.services.Items.Update(c.Request().Context(), *user, input); err != nil {
		return s.redirectWithFlash(c, "/items/"+itemID, "error", err.Error())
	}

	return s.redirectWithFlash(c, "/items/"+itemID, "success", "Item updated.")
}

func (s *Server) itemCommentPost(c *echo.Context) error {
	itemID := c.Param("itemID")
	form, err := s.readFormOrRedirect(c, "/items/"+itemID, "Could not read the comment form.")
	if err != nil {
		return err
	}
	if err := s.services.Items.AddComment(c.Request().Context(), *s.currentUser(c), itemID, form.Get("body")); err != nil {
		return s.redirectWithFlash(c, "/items/"+itemID, "error", err.Error())
	}
	return s.redirectWithFlash(c, "/items/"+itemID, "success", "Comment added.")
}

func (s *Server) itemEvidencePost(c *echo.Context) error {
	itemID := c.Param("itemID")
	form, err := s.readFormOrRedirect(c, "/items/"+itemID, "Could not read the evidence form.")
	if err != nil {
		return err
	}
	if err := s.services.Items.AddEvidenceLink(c.Request().Context(), *s.currentUser(c), itemID, form.Get("label"), form.Get("url")); err != nil {
		return s.redirectWithFlash(c, "/items/"+itemID, "error", err.Error())
	}
	return s.redirectWithFlash(c, "/items/"+itemID, "success", "Evidence link added.")
}

func (s *Server) readItemUpdateInput(form url.Values, itemID uuid.UUID, canManageAssessments bool) (service.UpdateItemInput, error) {
	status, err := service.ParseAssessmentItemStatus(form.Get("status"))
	if err != nil {
		return service.UpdateItemInput{}, err
	}

	priority, err := service.ParseItemPriority(form.Get("priority"))
	if err != nil {
		return service.UpdateItemInput{}, err
	}

	input := service.UpdateItemInput{
		ID:             itemID,
		Status:         status,
		Priority:       priority,
		OwnerUserID:    s.optionalUUID(form.Get("owner_user_id")),
		ReviewerUserID: s.optionalUUID(form.Get("reviewer_user_id")),
		Score:          s.optionalInt32(form.Get("score")),
		Notes:          textutil.TrimPtr(form.Get("notes")),
		BlockedReason:  textutil.TrimPtr(form.Get("blocked_reason")),
	}

	if !canManageAssessments {
		return input, nil
	}

	dueDate, err := textutil.ParseDateOnly(strings.TrimSpace(form.Get("due_date")))
	if err != nil {
		return service.UpdateItemInput{}, fmt.Errorf("Enter a valid due date.")
	}
	input.DueDate = dueDate
	return input, nil
}

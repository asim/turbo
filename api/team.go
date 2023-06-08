package api

import (
	"net/http"
	"strings"

	"github.com/asim/proxy-gpt/db"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Team who users are members of
type Team struct {
	gorm.Model
	ID          string `json:"id" valid:"required"`
	Name        string `json:"name" valid:"length(1|30)"`
	Description string `json:"description" valid:"length(1|256)"`
	OwnerID     string `json:"owner_id" gorm:"index"`
}

type TeamMember struct {
	gorm.Model
	TeamID string `json:"team_id" gorm:"index:idx_team_member_team_user"`
	UserID string `json:"user_id" gorm:"index:idx_team_member_team_user"`
}

// TeamIndexRequest for team/index
// Get token for userID and list all teams that user is in
// This is overkill as users will only have one team to start with but worth building for future
type TeamIndexRequest struct{}

// TeamIndexResponse for team/index
type TeamIndexResponse struct {
	Teams []Team `json:"teams"`
}

// TeamCreateRequest for team/create
type TeamCreateRequest struct {
	Name        string   `json:"name" valid:"length(1|30)"`
	Description string   `json:"description" valid:"length(1|256)"`
	Members     []string `json:"members"`
}

// TeamReadRequest for team/read
type TeamReadRequest struct {
	ID string `json:"id" valid:"required"`
}

type TeamReadResponse struct {
	Team
}

// TeamUpdateRequest for team/update
type TeamUpdateRequest struct {
	ID          string `json:"id" valid:"required"`
	Name        string `json:"name" valid:"length(1|30)"`
	Description string `json:"description" valid:"length(1|256)"`
}

// TeamUpdateResponse for team/update
type TeamUpdateResponse struct {
	Team
}

type TeamMembersInviteRequest struct {
	Username string `json:"username" valid:"required,length(6|254)"`
	TeamID   string `json:"team_id" valid:"required,length(1|254)"`
}

type TeamMembersInviteResponse struct{}

// TeamMembersRequest for team/users
// returns all users in an team
type TeamMembersRequest struct {
	ID string `json:"id" valid:"required"`
}

// TeamUsersResponse for team/users
type TeamMembersResponse struct {
	Users []User `json:"users"`
}

// TeamUsersCreateRequest for team/users/create
// adds a users to an team.
// TODO: permissions
type TeamMembersAddRequest struct {
	ID      string   `json:"id" valid:"required,length(1|254)"`
	UserIDs []string `json:"user_ids"`
}

type TeamMembersAddResponse struct{}

type TeamMembersRemoveRequest struct {
	ID      string   `json:"id" valid:"required,length(1|254)"`
	UserIDs []string `json:"user_ids"`
}

type TeamMembersRemoveResponse struct{}

type TeamDeleteRequest struct {
	ID string `json:"id" valid:"required"`
}

type TeamDeleteResponse struct {
	Team Team `json:"team"`
}

func TeamCreate(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := TeamCreateRequest{
		Name:        r.Form.Get("name"),
		Description: r.Form.Get("description"),
		Members:     r.Form["members"],
	}

	// Decode and validate the request
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// attempt to pull user session from context
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		// no session, don't proceed
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	team := Team{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     sess.UserID,
	}

	// create new team
	if err := CreateTeam(&team); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with created team
	respond(w, r, team)
}

func TeamDelete(w http.ResponseWriter, r *http.Request) {
	// Check user session
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse form
	r.ParseForm()

	// Decode request
	var req TeamDeleteRequest
	req.ID = r.Form.Get("id")

	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if user is an team owner
	var team Team
	if err := db.Where("id = ?", req.ID).First(&team).Error; err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// check ownership
	if team.OwnerID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// delete all team members
	if err := db.Where("team_id = ?", team.ID).Delete(&TeamMember{}).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Delete team
	if err := db.Delete(&team).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with TeamDeleteResponse
	respond(w, r, TeamDeleteResponse{Team: team})
}

func TeamMembers(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := TeamMembersRequest{
		ID: r.Form.Get("id"),
	}

	// Decode and validate the request
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get all members of the team
	var members []TeamMember

	if err := db.Model(&TeamMember{}).Where("team_id = ?", req.ID).Find(&members).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get details of all members
	var memberIDs []string
	for _, m := range members {
		memberIDs = append(memberIDs, m.UserID)
	}

	var memberDetails []User
	if err := db.Where("id IN ?", memberIDs).Find(&memberDetails).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with list of members
	respond(w, r, TeamMembersResponse{Users: memberDetails})
}

func TeamRead(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := TeamReadRequest{
		ID: r.Form.Get("id"),
	}

	// Decode and validate the request
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get team from database
	var team Team
	if err := db.Where("id = ?", req.ID).First(&team).Error; err != nil {
		http.Error(w, "team not found", 404)
		return
	}

	// Respond with team
	respond(w, r, TeamReadResponse{Team: team})
}

func TeamUpdate(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := TeamUpdateRequest{
		ID:          r.Form.Get("id"),
		Name:        r.Form.Get("name"),
		Description: r.Form.Get("description"),
	}

	// Check user session
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Decode and validate the request
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get team from database
	var team Team
	if err := db.Where("id = ?", req.ID).First(&team).Error; err != nil {
		http.Error(w, "team not found", 404)
		return
	}

	// check the owner matches
	if team.OwnerID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Update team with new name
	team.Name = req.Name
	team.Description = req.Description

	// Save team to database
	if err := db.Update(&team).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with updated team
	respond(w, r, TeamUpdateResponse{Team: team})
}

func TeamMembersAdd(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := TeamMembersAddRequest{
		ID:      r.Form.Get("id"),
		UserIDs: r.Form["user_ids"],
	}

	// Check user session
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Decode and validate the request
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get the team by ID
	var team Team
	if err := db.Where("id = ?", req.ID).First(&team).Error; err != nil {
		http.Error(w, "Team not found", http.StatusNotFound)
		return
	}

	// check owner matches
	if team.OwnerID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Add new members to team
	for _, userID := range req.UserIDs {
		teamMember := TeamMember{
			TeamID: team.ID,
			UserID: userID,
		}

		if err := db.Create(&teamMember).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Respond with success
	respond(w, r, TeamMembersAddResponse{})
}

func TeamIndex(w http.ResponseWriter, r *http.Request) {
	// Check user session
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get all TeamMembers for the current user
	var teamMembers []TeamMember
	if err := db.Where("user_id = ?", sess.UserID).Find(&teamMembers).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	teamIDs := make([]string, len(teamMembers))
	for i, om := range teamMembers {
		teamIDs[i] = om.TeamID
	}

	// Get Teams for each TeamMember
	var teams []Team
	if len(teamIDs) > 0 {
		if err := db.Where("id IN (?)", teamIDs).Find(&teams).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Respond with TeamIndexResponse
	respond(w, r, TeamIndexResponse{Teams: teams})
}

func TeamMembersRemove(w http.ResponseWriter, r *http.Request) {
	// Check user session
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get request parameters
	var req TeamMembersRemoveRequest
	req.ID = r.Form.Get("id")
	req.UserIDs = strings.Split(r.Form.Get("user_id"), ",")

	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// need 1 member to delete
	if len(req.UserIDs) == 0 {
		http.Error(w, "require at least 1 member", http.StatusBadRequest)
		return
	}

	// Get team
	var team Team
	if err := db.First(&team, "id = ?", req.ID).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get team member being deleted
	var teamMembers []TeamMember
	if err := db.Where("team_id = ?", req.ID).Find(&teamMembers).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if user is team owner
	if team.OwnerID != sess.UserID {
		// not team owner
		// can only remove self
		// must check if is team member

		// user can only remove the themselves
		if len(req.UserIDs) > 1 {
			http.Error(w, "can only remove self", http.StatusUnauthorized)
			return
		}

		// can only delete self
		if req.UserIDs[0] != sess.UserID {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// check if is an team member
		var isMember bool
		for _, member := range teamMembers {
			if member.UserID == sess.UserID {
				isMember = true
				break
			}
		}
		if !isMember {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// cannot delete members if there's only one left
	if len(teamMembers) <= 1 {
		http.Error(w, "team requires at least 1 member", http.StatusInternalServerError)
		return
	}

	// cannot remove owner
	for _, id := range req.UserIDs {
		if id == team.OwnerID {
			http.Error(w, "cannot delete team owner", http.StatusBadRequest)
			return
		}
	}

	// Delete team members
	if err := db.Where("team_id = ? AND user_id IN (?)", team.ID, req.UserIDs).Delete(&TeamMember{}).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respond(w, r, TeamMembersRemoveResponse{})
}

func CreateTeam(team *Team) error {
	if len(team.ID) == 0 {
		team.ID = uuid.New().String()
	}

	// Save team to database
	if err := db.Create(team).Error; err != nil {
		return err
	}

	teamMember := TeamMember{
		TeamID: team.ID,
		UserID: team.OwnerID,
	}

	if err := db.Create(&teamMember).Error; err != nil {
		return err
	}

	return nil
}

func GetTeamByID(id string) (*Team, error) {
	var team Team
	team.ID = id
	if err := db.First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func GetTeam(userID string) (*Team, error) {
	var teamMember TeamMember
	if err := db.Where("user_id = ?", userID).First(&teamMember).Error; err != nil {
		return nil, err
	}

	var team Team
	team.ID = teamMember.TeamID
	if err := db.First(&team).Error; err != nil {
		return nil, err
	}
	return &team, nil
}

func IsInTeam(teamID, userID string) bool {
	// Get all members of the team
	var member TeamMember

	if err := db.Where("team_id = ? AND user_id = ?", teamID, userID).First(&member).Error; err != nil {
		return false
	}

	// check it matches
	return member.UserID == userID && member.TeamID == teamID
}

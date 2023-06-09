package api

import (
	"net/http"
	"strings"

	"github.com/asim/proxy-gpt/db"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Group who users are members of
type Group struct {
	gorm.Model
	ID          string `json:"id" valid:"required"`
	Name        string `json:"name" valid:"length(1|30)"`
	Description string `json:"description" valid:"length(1|256)"`
	OwnerID     string `json:"owner_id" gorm:"index"`
}

type GroupMember struct {
	gorm.Model
	GroupID string `json:"group_id" gorm:"index:idx_group_member_group_user"`
	UserID string `json:"user_id" gorm:"index:idx_group_member_group_user"`
}

// GroupIndexRequest for group/index
// Get token for userID and list all groups that user is in
// This is overkill as users will only have one group to start with but worth building for future
type GroupIndexRequest struct{}

// GroupIndexResponse for group/index
type GroupIndexResponse struct {
	Groups []Group `json:"groups"`
}

// GroupCreateRequest for group/create
type GroupCreateRequest struct {
	Name        string   `json:"name" valid:"length(1|30)"`
	Description string   `json:"description" valid:"length(1|256)"`
	Members     []string `json:"members"`
}

// GroupReadRequest for group/read
type GroupReadRequest struct {
	ID string `json:"id" valid:"required"`
}

type GroupReadResponse struct {
	Group
}

// GroupUpdateRequest for group/update
type GroupUpdateRequest struct {
	ID          string `json:"id" valid:"required"`
	Name        string `json:"name" valid:"length(1|30)"`
	Description string `json:"description" valid:"length(1|256)"`
}

// GroupUpdateResponse for group/update
type GroupUpdateResponse struct {
	Group
}

type GroupMembersInviteRequest struct {
	Username string `json:"username" valid:"required,length(6|254)"`
	GroupID   string `json:"group_id" valid:"required,length(1|254)"`
}

type GroupMembersInviteResponse struct{}

// GroupMembersRequest for group/users
// returns all users in an group
type GroupMembersRequest struct {
	ID string `json:"id" valid:"required"`
}

// GroupUsersResponse for group/users
type GroupMembersResponse struct {
	Users []User `json:"users"`
}

// GroupUsersCreateRequest for group/users/create
// adds a users to an group.
// TODO: permissions
type GroupMembersAddRequest struct {
	ID      string   `json:"id" valid:"required,length(1|254)"`
	UserIDs []string `json:"user_ids"`
}

type GroupMembersAddResponse struct{}

type GroupMembersRemoveRequest struct {
	ID      string   `json:"id" valid:"required,length(1|254)"`
	UserIDs []string `json:"user_ids"`
}

type GroupMembersRemoveResponse struct{}

type GroupDeleteRequest struct {
	ID string `json:"id" valid:"required"`
}

type GroupDeleteResponse struct {
	Group Group `json:"group"`
}

func GroupCreate(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := GroupCreateRequest{
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

	group := Group{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     sess.UserID,
	}

	// create new group
	if err := CreateGroup(&group); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with created group
	respond(w, r, group)
}

func GroupDelete(w http.ResponseWriter, r *http.Request) {
	// Check user session
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse form
	r.ParseForm()

	// Decode request
	var req GroupDeleteRequest
	req.ID = r.Form.Get("id")

	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Check if user is an group owner
	var group Group
	if err := db.Where("id = ?", req.ID).First(&group).Error; err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// check ownership
	if group.OwnerID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// delete all group members
	if err := db.Where("group_id = ?", group.ID).Delete(&GroupMember{}).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Delete group
	if err := db.Delete(&group).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with GroupDeleteResponse
	respond(w, r, GroupDeleteResponse{Group: group})
}

func GroupMembers(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := GroupMembersRequest{
		ID: r.Form.Get("id"),
	}

	// Decode and validate the request
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get all members of the group
	var members []GroupMember

	if err := db.Model(&GroupMember{}).Where("group_id = ?", req.ID).Find(&members).Error; err != nil {
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
	respond(w, r, GroupMembersResponse{Users: memberDetails})
}

func GroupRead(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := GroupReadRequest{
		ID: r.Form.Get("id"),
	}

	// Decode and validate the request
	if err := decode(r, &req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Get group from database
	var group Group
	if err := db.Where("id = ?", req.ID).First(&group).Error; err != nil {
		http.Error(w, "group not found", 404)
		return
	}

	// Respond with group
	respond(w, r, GroupReadResponse{Group: group})
}

func GroupUpdate(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := GroupUpdateRequest{
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

	// Get group from database
	var group Group
	if err := db.Where("id = ?", req.ID).First(&group).Error; err != nil {
		http.Error(w, "group not found", 404)
		return
	}

	// check the owner matches
	if group.OwnerID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Update group with new name
	group.Name = req.Name
	group.Description = req.Description

	// Save group to database
	if err := db.Update(&group).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Respond with updated group
	respond(w, r, GroupUpdateResponse{Group: group})
}

func GroupMembersAdd(w http.ResponseWriter, r *http.Request) {
	// Parse form and fill request with form values
	r.ParseForm()
	req := GroupMembersAddRequest{
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

	// Get the group by ID
	var group Group
	if err := db.Where("id = ?", req.ID).First(&group).Error; err != nil {
		http.Error(w, "Group not found", http.StatusNotFound)
		return
	}

	// check owner matches
	if group.OwnerID != sess.UserID {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Add new members to group
	for _, userID := range req.UserIDs {
		groupMember := GroupMember{
			GroupID: group.ID,
			UserID: userID,
		}

		if err := db.Create(&groupMember).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Respond with success
	respond(w, r, GroupMembersAddResponse{})
}

func GroupIndex(w http.ResponseWriter, r *http.Request) {
	// Check user session
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get all GroupMembers for the current user
	var groupMembers []GroupMember
	if err := db.Where("user_id = ?", sess.UserID).Find(&groupMembers).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	groupIDs := make([]string, len(groupMembers))
	for i, om := range groupMembers {
		groupIDs[i] = om.GroupID
	}

	// Get Groups for each GroupMember
	var groups []Group
	if len(groupIDs) > 0 {
		if err := db.Where("id IN (?)", groupIDs).Find(&groups).Error; err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Respond with GroupIndexResponse
	respond(w, r, GroupIndexResponse{Groups: groups})
}

func GroupMembersRemove(w http.ResponseWriter, r *http.Request) {
	// Check user session
	sess, ok := r.Context().Value(Session{}).(*Session)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	// Get request parameters
	var req GroupMembersRemoveRequest
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

	// Get group
	var group Group
	if err := db.First(&group, "id = ?", req.ID).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Get group member being deleted
	var groupMembers []GroupMember
	if err := db.Where("group_id = ?", req.ID).Find(&groupMembers).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Check if user is group owner
	if group.OwnerID != sess.UserID {
		// not group owner
		// can only remove self
		// must check if is group member

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

		// check if is an group member
		var isMember bool
		for _, member := range groupMembers {
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
	if len(groupMembers) <= 1 {
		http.Error(w, "group requires at least 1 member", http.StatusInternalServerError)
		return
	}

	// cannot remove owner
	for _, id := range req.UserIDs {
		if id == group.OwnerID {
			http.Error(w, "cannot delete group owner", http.StatusBadRequest)
			return
		}
	}

	// Delete group members
	if err := db.Where("group_id = ? AND user_id IN (?)", group.ID, req.UserIDs).Delete(&GroupMember{}).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respond(w, r, GroupMembersRemoveResponse{})
}

func CreateGroup(group *Group) error {
	if len(group.ID) == 0 {
		group.ID = uuid.New().String()
	}

	// Save group to database
	if err := db.Create(group).Error; err != nil {
		return err
	}

	groupMember := GroupMember{
		GroupID: group.ID,
		UserID: group.OwnerID,
	}

	if err := db.Create(&groupMember).Error; err != nil {
		return err
	}

	return nil
}

func GetGroupByID(id string) (*Group, error) {
	var group Group
	group.ID = id
	if err := db.First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func GetGroup(userID string) (*Group, error) {
	var groupMember GroupMember
	if err := db.Where("user_id = ?", userID).First(&groupMember).Error; err != nil {
		return nil, err
	}

	var group Group
	group.ID = groupMember.GroupID
	if err := db.First(&group).Error; err != nil {
		return nil, err
	}
	return &group, nil
}

func IsInGroup(groupID, userID string) bool {
	// Get all members of the group
	var member GroupMember

	if err := db.Where("group_id = ? AND user_id = ?", groupID, userID).First(&member).Error; err != nil {
		return false
	}

	// check it matches
	return member.UserID == userID && member.GroupID == groupID
}

package users

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/scott-ace-newton/users-rw-sql/notification"
	"github.com/scott-ace-newton/users-rw-sql/persistence"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"net/url"
	"strings"
)

const msgTemplate = "{\"message\": \"%s\"}"

//Check models a very basic healthcheck
// swagger:model Check
type Check struct {
	System string `json:"system"`
	Status string `json:"status"`
}

//UsersHandler stores configured sql and queue clients
type UsersHandler struct {
	sqlClient persistence.Clienter
	queueClient notification.QueueClient
}

//NewUsersHandler returns handler with configured sql and queue clients
func NewUsersHandler(sqlClient persistence.Clienter, queueClient notification.QueueClient) UsersHandler {
	return UsersHandler{
		sqlClient: sqlClient,
		queueClient: queueClient,
	}
}

//RegisterHandlers registers application endpoints
func (h *UsersHandler) RegisterHandlers(router *mux.Router) {
	log.Info("registering handlers")
	editDeleteUserHandler := handlers.MethodHandler{
		"PATCH": http.HandlerFunc(h.EditUser),
		"DELETE": http.HandlerFunc(h.DeleteUser),
	}
	addGetUserHandler := handlers.MethodHandler{
		"PUT": http.HandlerFunc(h.AddUser),
		"GET": http.HandlerFunc(h.GetRecords),
	}
	healthHandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(h.IsHealthy),
	}

	router.Handle("/users/{userID}", editDeleteUserHandler)
	router.Handle("/users", addGetUserHandler)
	router.Handle("/__health", healthHandler)
}

// AddUser swagger:route PUT /users users addUser
// ---
// summary: Add users
// description: Add user records to DB
// parameters:
// - name: userID
//   in: body
//   description: users uuid
//   type: string
//   required: true
// - name: firstName
//   in: body
//   description: users first name
//   type: string
//   required: true
// - name: lastName
//   in: body
//   description: users last name
//   type: string
//   required: true
// - name: emailAddress
//   in: body
//   description: users email address
//   type: string
//   required: true
// - name: password
//   in: body
//   description: users password
//   type: string
//   required: true
// - name: nickname
//   in: body
//   description: users nickname
//   type: string
//   required: true
// - name: country
//   in: body
//   description: users country
//   type: string
//   required: true
//201: created
//400: badRequest
//409: conflict
//500: internal
func (h *UsersHandler) AddUser(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Add("Content-Type", "application/json")
	var body io.Reader = request.Body
	dec := json.NewDecoder(body)

	ur := persistence.UserRecord{}
	if err := dec.Decode(&ur); err != nil {
		log.WithError(err).Error("could not decode request body")
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not decode request body"))
		return
	}

	//generates unique user ID using email
	ur.UserID = uuid.NewMD5(uuid.UUID{}, []byte(ur.EmailAddress)).String()
	log.Debugf("generated ID: %s for new user with email: %s", ur.UserID, ur.EmailAddress)

	switch h.sqlClient.CreateRecord(ur) {
	case persistence.CREATED:
		h.queueClient.AddMessageToQueue(persistence.Message{
			Type: "USER_CREATED",
			UserID: ur.UserID,
		})
		writer.WriteHeader(http.StatusCreated)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "created user with ID: " + ur.UserID))
	case persistence.ALREADY_EXISTS:
		writer.WriteHeader(http.StatusConflict)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, fmt.Sprintf("user with email: %s already exists in db!", ur.EmailAddress)))
	default:
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not add user to db"))
	}
}

// swagger:operation PATCH /users/{userID} users editUser
// ---
// summary: Modify users
// description: Modify provided user fields for the specified UserID
// parameters:
// - name: userID
//   in: path
//   description: users uuid
//   type: string
//   required: true
// - name: firstName
//   in: body
//   description: users first name
//   type: string
//   required: false
// - name: lastName
//   in: body
//   description: users last name
//   type: string
//   required: false
// - name: password
//   in: body
//   description: users password
//   type: string
//   required: false
// - name: nickname
//   in: body
//   description: users nickname
//   type: string
//   required: false
// - name: country
//   in: body
//   description: users country
//   type: string
//   required: false
// responses:
//   200: ok
//   400: badRequest
//   404: notFound
//   500: internal
func (h *UsersHandler) EditUser(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Add("Content-Type", "application/json")
	vars := mux.Vars(request)
	userID := vars["userID"]

	var body io.Reader = request.Body
	dec := json.NewDecoder(body)
	ur := persistence.UserRecord{}
	if err := dec.Decode(&ur); err != nil {
		log.WithError(err).Error("could not decode request body")
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not decode request body"))
		return
	}

	if ur.EmailAddress != "" {
		log.WithField("UserID", userID).Error( "users are currently unable to change their email address")
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "users are currently unable to change their email address"))
		return
	}

	updates, nicknameChanged := extractFieldsToUpdate(ur)
	if len(updates) == 0 {
		log.WithField("UserID", userID).Infof( "supplied fields are not valid for update in request body: %v", body)
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "supplied fields are not valid for update"))
		return
	}

	switch h.sqlClient.UpdateRecord(userID, updates) {
	case persistence.UPDATED:
		if nicknameChanged {
			h.queueClient.AddMessageToQueue(persistence.Message{
				Type: "NICKNAME_CHANGED",
				UserID: userID,
				Nickname: updates["nickname"],
			})
		}
		writer.WriteHeader(http.StatusOK)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "updated user: " + userID))
	case persistence.NOT_FOUND:
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not update user: " + userID + " as they did not exist"))
	default:
		msg := fmt.Sprintf("could not update user: %s", userID)
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, msg))
	}
}

func extractFieldsToUpdate(ur persistence.UserRecord) (map[string]string, bool) {
	updates := make(map[string]string)
	var nicknameChanged bool
	if ur.FirstName != "" {
		updates["first_name"] = ur.FirstName
	}
	if ur.LastName != "" {
		updates["last_name"] = ur.LastName
	}
	if ur.Password != "" {
		updates["password"] = ur.Password
	}
	if ur.NickName != "" {
		updates["nickname"] = ur.NickName
		nicknameChanged = true
	}
	if ur.Country != "" {
		updates["country"] = ur.Country
	}
	return updates, nicknameChanged
}

// swagger:operation GET /users users getUser
// ---
// summary: Return userList
// description: Returns list of users matching specified criteria
// parameters:
// - name: userID
//   in: query
//   description: users uuid
//   type: string
//   required: false
// - name: firstName
//   in: query
//   description: users first name
//   type: string
//   required: false
// - name: lastName
//   in: query
//   description: users last name
//   type: string
//   required: false
// - name: emailAddress
//   in: query
//   description: users email address
//   type: string
//   required: false
// - name: nickname
//   in: query
//   description: users nickname
//   type: string
//   required: false
// - name: country
//   in: query
//   description: users country
//   type: string
//   required: false
// responses:
//   200: ok
//   400: badRequest
//   404: notFound
//   422: unprocessable
//   500: internal
func (h *UsersHandler) GetRecords(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Add("Content-Type", "application/json")

	params, err := url.ParseQuery(request.URL.RawQuery)
	if err != nil {
		log.WithError(err).Errorf("malformed request query: %v", request.URL.RawQuery)
		writer.WriteHeader(http.StatusUnprocessableEntity)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "malformed request query"))
		return
	}

	if len(params) == 0 {
		log.Info("no search criteria were supplied on request")
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "no url params supplied as criteria by which to search for matching users"))
		return
	}

	searchCriteria := make(map[string]string)
	for k, v := range params {
		newKey := filterQueryParams(k)
		if newKey != "" {
			//remove quotes from values
			searchCriteria[newKey] = strings.Replace(v[0], `"`, "", -1)
		}
	}

	if len(searchCriteria) == 0 {
		log.Infof("supplied request params %s are invalid", params)
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "supplied request params are invalid; valid params are [userID, firstName, lastName, emailAddress, nickname, country]"))
		return
	}

	users, retrievalStatus := h.sqlClient.RetrieveRecords(searchCriteria)
	switch retrievalStatus {
	case persistence.OK:
		writer.WriteHeader(http.StatusOK)
		enc := json.NewEncoder(writer)
		if err := enc.Encode(users); err != nil {
			log.WithError(err).Error("could not encode returned payload")
			writer.WriteHeader(http.StatusInternalServerError)
			fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not process request"))
			return
		}
	case persistence.NOT_FOUND:
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "found no users matching specified criteria"))
	default:
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not process request"))
	}
}

func filterQueryParams(key string) string {
	switch key {
	case "userID":
		return "user_id"
	case "firstName":
		return "first_name"
	case "lastName":
		return "last_name"
	case "emailAddress":
		return "email"
	case "nickname":
		return "nickname"
	case "country":
		return "country"
	default:
		log.Errorf("supplied param %s is invalid", key)
		return ""
	}
	return ""
}

// swagger:operation DELETE /users/{userID} users deleteUser
// ---
// summary: Delete users
// description: Delete user with matching UserID from DB
// parameters:
// - name: userID
//   in: path
//   description: users uuid
//   type: string
//   required: true
// responses:
//   204: noContent
//   404: notFound
//   500: internal
func (h *UsersHandler) DeleteUser(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Add("Content-Type", "application/json")
	vars := mux.Vars(request)
	userID := vars["userID"]
	switch h.sqlClient.DeleteRecord(userID) {
	case persistence.DELETED:
		writer.WriteHeader(http.StatusNoContent)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "user record deleted"))
	case persistence.NOT_FOUND:
		writer.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "user does not exist"))
	default:
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not process delete request"))
	}
}

//IsHealthy swagger:route GET /__health isHealthy
//
//Returns health of system
//
//Responses:
//
//200: ok
//503: unavailable
func (h *UsersHandler) IsHealthy(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Add("Content-Type", "application/json")
	var checks []Check

	if h.sqlClient.ActiveConnection() {
		checks = append(checks, Check{"sqlDB", "healthy"})
	} else {
		checks = append(checks, Check{"sqlDB", "unhealthy"})
	}

	if h.queueClient.QueueIsWritable() {
		checks = append(checks, Check{"msgQueue", "healthy"})
	} else {
		checks = append(checks, Check{"msgQueue", "unhealthy"})
	}


	enc := json.NewEncoder(writer)
	enc.Encode(checks)
}
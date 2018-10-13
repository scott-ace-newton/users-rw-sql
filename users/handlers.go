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
)

const (
	msgTemplate = "{\"message\": \"%s\"}"
	CreatedEvent = "USER_CREATED"
	NickNameChangedEvent   = "NICKNAME_CHANGED"
)


type UsersHandler struct {
	sqlClient persistence.Clienter
	queueClient notification.QueueClient
}

func NewUsersHandler(sqlClient persistence.Clienter, queueClient notification.QueueClient) UsersHandler {
	return UsersHandler{
		sqlClient: sqlClient,
		queueClient: queueClient,
	}
}

func (h *UsersHandler) RegisterHandlers(router *mux.Router) {
	log.Info("registering handlers")
	addUserHandler := handlers.MethodHandler{
		"PUT": http.HandlerFunc(h.AddUser),
	}
	editUserHandler := handlers.MethodHandler{
		"PATCH": http.HandlerFunc(h.EditUser),
	}
	getUserHandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(h.GetRecords),
	}
	removeUserHandler := handlers.MethodHandler{
		"DELETE": http.HandlerFunc(h.DeleteUser),
	}
	healthHandler := handlers.MethodHandler{
		"GET": http.HandlerFunc(h.IsHealthy),
	}

	// These paths need to actually be the concept type
	router.Handle("/addUser", addUserHandler)
	router.Handle("/user/{userID}", editUserHandler)
	router.Handle("/user", getUserHandler)
	router.Handle("/user/{userID}", removeUserHandler)
	router.Handle("/__health", healthHandler)
}

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
			Type: CreatedEvent,
			UserID: ur.UserID,
		})
		writer.WriteHeader(http.StatusCreated)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "created user with ID: " + ur.UserID))
	case persistence.ALREADY_EXISTS:
		writer.WriteHeader(http.StatusConflict)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, fmt.Sprintf("user with email: %s already exists in DB!", ur.EmailAddress)))
	default:
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not add user to DB"))
	}
}

func (h *UsersHandler) EditUser(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Add("Content-Type", "application/json")
	vars := mux.Vars(request)
	userID := vars["userID"]

	var body io.Reader = request.Body
	dec := json.NewDecoder(body)

	ur := persistence.UserRecord{}
	if err := dec.Decode(&ur); err != nil {
		log.WithError(err).Error("could not decode request body")
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not decode request body"))
		return
	}

	updates, nicknameChanged := extractFieldsToUpdate(ur)
	if len(updates) == 0 {
		log.WithField("UserID", userID).Error(fmt.Sprintf( "supplied fields are not valid for update in request body: %v", body))
		writer.WriteHeader(http.StatusBadRequest)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "supplied fields are not valid for update"))
		return
	}

	switch h.sqlClient.UpdateRecord(userID, updates) {
	case persistence.UPDATED:
		if nicknameChanged {
			h.queueClient.AddMessageToQueue(persistence.Message{
				Type: NickNameChangedEvent,
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
	if ur.EmailAddress != "" {
		updates["email"] = ur.EmailAddress
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
		searchCriteria[filterQueryParams(k)] = v[0]
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
	case "email":
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

func (h *UsersHandler) DeleteUser(writer http.ResponseWriter, request *http.Request) {
	writer.Header().Add("Content-Type", "application/json")
	vars := mux.Vars(request)
	userID := vars["userID"]
	switch h.sqlClient.DeleteRecord(userID) {
	case persistence.DELETED:
		writer.WriteHeader(http.StatusNoContent)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "user record deleted"))
	case persistence.NOT_FOUND:
		writer.WriteHeader(http.StatusNoContent)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "user does not exist"))
	default:
		writer.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "could not process delete request"))
	}
}

func (h *UsersHandler) IsHealthy(writer http.ResponseWriter, request *http.Request) {
	//TODO do properly
	if h.sqlClient.ActiveConnection() {
		writer.WriteHeader(http.StatusOK)
		fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "app is healthy"))
	}
	writer.WriteHeader(http.StatusServiceUnavailable)
	fmt.Fprintln(writer, fmt.Sprintf(msgTemplate, "app is unhealthy"))
}
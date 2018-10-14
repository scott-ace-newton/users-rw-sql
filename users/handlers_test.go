package users

import (
	"fmt"
	"github.com/gorilla/mux"
	"github.com/scott-ace-newton/users-rw-sql/notification"
	"github.com/scott-ace-newton/users-rw-sql/persistence"
	"github.com/stretchr/testify/assert"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var johnSmithJSON = `{
  "userID": "e41e62c8-6cf2-4fd7-a88b-41b86fcaa34d",
  "firstName": "John",
  "lastName": "Smith",
  "emailAddress": "john.smith@gmail.com",
  "password": "password1",
  "nickname": "smithy12345",
  "country": "UK"
}`

var johnSmithUser = persistence.UserRecord{
	UserID: "e41e62c8-6cf2-4fd7-a88b-41b86fcaa34d",
	FirstName: "John",
	LastName: "Smith",
	EmailAddress: "john.smith@gmail.com",
	Password: "password1",
	NickName: "smithy12345",
	Country: "UK",
}

var updateNickname = `{
  "nickname": "KingSmithy"
}`

var updateAddress = `{
  "address": "742 Evergreen Terrace"
}`

var updateEmail = `{
  "emailAddress": "KingSmithy@gmail.com"
}`

func TestPutHandler(t *testing.T) {
	qc := notification.NewQueueClient("/dev/null")
	assert := assert.New(t)
	tests := []struct {
		name        string
		sqlClient   *mockSQLClient
		reqBody     string
		statusCode  int
		body        string
	}{
		{
			name:       "Can add valid user to db",
			sqlClient:  &mockSQLClient{persistence.CREATED, nil},
			reqBody:    johnSmithJSON,
			statusCode: http.StatusCreated,
			body:       fmt.Sprintf(msgTemplate + "\n", "created user with ID: 3f685356-02a0-3c55-8b8d-c8bac4b79426"),
		},
		{
			name:       "Cannot re-create existing user",
			sqlClient:  &mockSQLClient{persistence.ALREADY_EXISTS, nil},
			reqBody:    johnSmithJSON,
			statusCode: http.StatusConflict,
			body:       fmt.Sprintf(msgTemplate + "\n", "user with email: john.smith@gmail.com already exists in db!"),
		},
		{
			name: "Error on invalid json",
			sqlClient: &mockSQLClient{persistence.BACKEND_ERROR, nil},
			reqBody: `{`,
			statusCode: http.StatusBadRequest,
			body:       fmt.Sprintf(msgTemplate + "\n", "could not decode request body"),
		},
		{
			name:       "Error on unable to write to db",
			sqlClient:  &mockSQLClient{persistence.BACKEND_ERROR, nil},
			reqBody:    johnSmithJSON,
			statusCode: http.StatusInternalServerError,
			body:       fmt.Sprintf(msgTemplate + "\n", "could not add user to db"),
		},
	}

	for _, test := range tests {
		r := mux.NewRouter()
		handler := NewUsersHandler(test.sqlClient, qc)
		handler.RegisterHandlers(r)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, newRequest("PUT", "/users", strings.NewReader(test.reqBody)))
		assert.Equal(test.statusCode, rec.Code, fmt.Sprintf("%s: Wrong response code, was %d, should be %d", test.name, rec.Code, test.statusCode))
		assert.Equal(test.body, rec.Body.String(), fmt.Sprintf("%s: Wrong body", test.name))
	}
}

func TestGetHandler(t *testing.T) {
	qc := notification.NewQueueClient("/dev/null")
	assert := assert.New(t)
	tests := []struct {
		name       string
		sqlClient  *mockSQLClient
		reqURL     string
		statusCode int
		body       string
	}{
		{
			name:       "Can return matching user from db",
			sqlClient:  &mockSQLClient{persistence.OK, []persistence.UserRecord{johnSmithUser}},
			reqURL:     "/users?userID=3f685356-02a0-3c55-8b8d-c8bac4b79426",
			statusCode: http.StatusOK,
			body:       convertBody(johnSmithJSON),
		},
		{
			name:       "Will return empty list when no matching users in db",
			sqlClient:  &mockSQLClient{persistence.NOT_FOUND, []persistence.UserRecord{}},
			reqURL:     "/users?userID=3f685356-02a0-3c55-8b8d-c8bac4b79426",
			statusCode: http.StatusNotFound,
			body:       fmt.Sprintf(msgTemplate + "\n", "found no users matching specified criteria"),
		},
		{
			name:       "Error on malformed request url",
			sqlClient:  &mockSQLClient{persistence.OK, []persistence.UserRecord{johnSmithUser}},
			reqURL:     `/users?%`,
			statusCode: http.StatusUnprocessableEntity,
			body:       fmt.Sprintf(msgTemplate + "\n", "malformed request query"),
		},
		{
			name:       "Error on no query params",
			sqlClient:  &mockSQLClient{persistence.OK, []persistence.UserRecord{johnSmithUser}},
			reqURL:     `/users`,
			statusCode: http.StatusBadRequest,
			body:       fmt.Sprintf(msgTemplate + "\n", "no url params supplied as criteria by which to search for matching users"),
		},
		{
			name:       "Error on unable to search records in db",
			sqlClient:  &mockSQLClient{persistence.BACKEND_ERROR, []persistence.UserRecord{}},
			reqURL:     "/users?userID=3f685356-02a0-3c55-8b8d-c8bac4b79426",
			statusCode: http.StatusInternalServerError,
			body:       fmt.Sprintf(msgTemplate + "\n", "could not process request"),
		},
	}

	for _, test := range tests {
		r := mux.NewRouter()
		handler := NewUsersHandler(test.sqlClient, qc)
		handler.RegisterHandlers(r)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, newRequest("GET", test.reqURL, nil))
		assert.Equal(test.statusCode, rec.Code, fmt.Sprintf("%s: Wrong response code, was %d, should be %d", test.name, rec.Code, test.statusCode))
		assert.Equal(test.body, rec.Body.String(), fmt.Sprintf("%s: Wrong body", test.name))
	}
}

func TestEditHandler(t *testing.T) {
	qc := notification.NewQueueClient("/dev/null")
	assert := assert.New(t)
	tests := []struct {
		name        string
		sqlClient   *mockSQLClient
		reqBody     string
		statusCode  int
		body        string
	}{
		{
			name: "Can edit existing user in db",
			sqlClient: &mockSQLClient{persistence.UPDATED, nil},
			reqBody: updateNickname,
			statusCode: http.StatusOK,
			body:       fmt.Sprintf(msgTemplate + "\n", "updated user: 3f685356-02a0-3c55-8b8d-c8bac4b79426"),
		},
		{
			name: "Cannot edit non-existing user in db",
			sqlClient: &mockSQLClient{persistence.NOT_FOUND, nil},
			reqBody: updateNickname,
			statusCode: http.StatusNotFound,
			body:       fmt.Sprintf(msgTemplate + "\n", "could not update user: 3f685356-02a0-3c55-8b8d-c8bac4b79426 as they did not exist"),
		},
		{
			name: "Error on invalid request body",
			sqlClient: &mockSQLClient{persistence.NOT_FOUND, nil},
			reqBody: "{,}",
			statusCode: http.StatusBadRequest,
			body:       fmt.Sprintf(msgTemplate + "\n", "could not decode request body"),
		},
		{
			name: "Error on invalid request params",
			sqlClient: &mockSQLClient{persistence.NOT_FOUND, nil},
			reqBody: updateAddress,
			statusCode: http.StatusBadRequest,
			body:       fmt.Sprintf(msgTemplate + "\n", "supplied fields are not valid for update"),
		},
		{
			name: "Error on request to update email",
			sqlClient: &mockSQLClient{persistence.NOT_FOUND, nil},
			reqBody: updateEmail,
			statusCode: http.StatusBadRequest,
			body:       fmt.Sprintf(msgTemplate + "\n", "users are currently unable to change their email address"),
		},
		{
			name: "Error on unable to update user in db",
			sqlClient: &mockSQLClient{persistence.BACKEND_ERROR, nil},
			reqBody: updateNickname,
			statusCode: http.StatusInternalServerError,
			body:       fmt.Sprintf(msgTemplate + "\n", "could not update user: 3f685356-02a0-3c55-8b8d-c8bac4b79426"),
		},
	}

	for _, test := range tests {
		r := mux.NewRouter()
		handler := NewUsersHandler(test.sqlClient, qc)
		handler.RegisterHandlers(r)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, newRequest("PATCH", "/users/3f685356-02a0-3c55-8b8d-c8bac4b79426", strings.NewReader(test.reqBody)))
		assert.Equal(test.statusCode, rec.Code, fmt.Sprintf("%s: Wrong response code, was %d, should be %d", test.name, rec.Code, test.statusCode))
		assert.Equal(test.body, rec.Body.String(), fmt.Sprintf("%s: Wrong body", test.name))
	}
}

func TestDeleteHandler(t *testing.T) {
	qc := notification.NewQueueClient("/dev/null")
	assert := assert.New(t)
	tests := []struct {
		name       string
		sqlClient  *mockSQLClient
		reqURL     string
		statusCode int
		body       string
	}{
		{
			name:       "Can delete existing user in db",
			sqlClient:  &mockSQLClient{persistence.DELETED, nil},
			reqURL:     "/users/3f685356-02a0-3c55-8b8d-c8bac4b79426",
			statusCode: http.StatusNoContent,
			body:       fmt.Sprintf(msgTemplate + "\n", "user record deleted"),
		},
		{
			name:       "Cannot delete non-existing user in db",
			sqlClient:  &mockSQLClient{persistence.NOT_FOUND, nil},
			reqURL:     "/users/3f685356-02a0-3c55-8b8d-c8bac4b79426",
			statusCode: http.StatusNotFound,
			body:       fmt.Sprintf(msgTemplate + "\n", "user does not exist"),
		},
		{
			name:       "Erorr when unable to delete records in db",
			sqlClient:  &mockSQLClient{persistence.BACKEND_ERROR, nil},
			reqURL:     "/users/3f685356-02a0-3c55-8b8d-c8bac4b79426",
			statusCode: http.StatusInternalServerError,
			body:       fmt.Sprintf(msgTemplate + "\n", "could not process delete request"),
		},
	}

	for _, test := range tests {
		r := mux.NewRouter()
		handler := NewUsersHandler(test.sqlClient, qc)
		handler.RegisterHandlers(r)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, newRequest("DELETE", test.reqURL, nil))
		assert.Equal(test.statusCode, rec.Code, fmt.Sprintf("%s: Wrong response code, was %d, should be %d", test.name, rec.Code, test.statusCode))
		assert.Equal(test.body, rec.Body.String(), fmt.Sprintf("%s: Wrong body", test.name))
	}
}

func newRequest(method, url string, body io.Reader) *http.Request {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		panic(err)
	}
	return req
}

func convertBody(user string) string {
	//remove spaces
	user2 := strings.Replace(user, " ", "", -1)
	//remove new lines
	user3 := strings.Replace(user2, "\n", "", -1)
	//convert to array and add newline
	return "[" + user3 + "]\n"
}

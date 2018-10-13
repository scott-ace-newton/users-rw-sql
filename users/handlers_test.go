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

var johnSmithJson = `{
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

func TestPutHandler(t *testing.T) {
	qc := notification.NewQueueClient("/dev/null")
	assert := assert.New(t)
	tests := []struct {
		name        string
		sqlClient   *mockSqlClient
		reqBody     string
		statusCode  int
		body        string
	}{
		{
			name: "Can add valid user to DB",
			sqlClient: &mockSqlClient{persistence.CREATED, nil},
			reqBody: johnSmithJson,
			statusCode: http.StatusCreated,
			body:       fmt.Sprintf(msgTemplate + "\n", "created user with ID: 3f685356-02a0-3c55-8b8d-c8bac4b79426"),
		},
		{
			name: "Cannot re-create existing user",
			sqlClient: &mockSqlClient{persistence.ALREADY_EXISTS, nil},
			reqBody: johnSmithJson,
			statusCode: http.StatusConflict,
			body:       fmt.Sprintf(msgTemplate + "\n", "user with email: john.smith@gmail.com already exists in DB!"),
		},
		{
			name: "Error on invalid json",
			sqlClient: &mockSqlClient{persistence.BACKEND_ERROR, nil},
			reqBody: `{`,
			statusCode: http.StatusBadRequest,
			body:       fmt.Sprintf(msgTemplate + "\n", "could not decode request body"),
		},
		{
			name: "Error on unable to write to DB",
			sqlClient: &mockSqlClient{persistence.BACKEND_ERROR, nil},
			reqBody: johnSmithJson,
			statusCode: http.StatusInternalServerError,
			body:       fmt.Sprintf(msgTemplate + "\n", "could not add user to DB"),
		},
	}

	for _, test := range tests {
		r := mux.NewRouter()
		handler := UsersHandler{test.sqlClient, qc}
		handler.RegisterHandlers(r)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, newRequest("PUT", "/addUser", strings.NewReader(test.reqBody)))
		assert.Equal(test.statusCode, rec.Code, fmt.Sprintf("%s: Wrong response code, was %d, should be %d", test.name, rec.Code, test.statusCode))
		assert.Equal(test.body, rec.Body.String(), fmt.Sprintf("%s: Wrong body", test.name))
	}
}


func TestGetHandler(t *testing.T) {
	qc := notification.NewQueueClient("/dev/null")
	assert := assert.New(t)
	tests := []struct {
		name        string
		sqlClient   *mockSqlClient
		reqUrl     string
		statusCode  int
		body        string
	}{
		{
			name: "Can return matching user from DB",
			sqlClient: &mockSqlClient{persistence.OK, []persistence.UserRecord{johnSmithUser}},
			reqUrl: "/user?userID=3f685356-02a0-3c55-8b8d-c8bac4b79426",
			statusCode: http.StatusOK,
			body:       convertBody(johnSmithJson),
		},
	}

	for _, test := range tests {
		r := mux.NewRouter()
		handler := UsersHandler{test.sqlClient, qc}
		handler.RegisterHandlers(r)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, newRequest("GET", test.reqUrl, nil))
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

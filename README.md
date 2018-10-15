# users-rw-sql

## Introduction
Users Rw Sql is a RESTful API allowing for CRUD operations on User data

## Installation
Download source code

        go get -u github.com/scott-ace-newton/users-rw-sql
        cd $GOPATH/src/github.com/scott-ace-newton/users-rw-sql
        
Start docker container with SQL server and application

        docker-compose build
        docker-compose up              
        
Run the tests. FYI the tests clear down the DB after running.
        
        go test ./... -race -cover

Clear down container when finished

        docker-compose down

## Service endpoints

    PUT /users   - adds user records to DB
      {
        "firstName": "John",
        "lastName": "Smith",
        "emailAddress": "JohnSmith@gmail.com",
        "password": "password1",
        "nickname": "smithy12345",
        "country": "UK"
      }
      
    GET /users   - returns user records matching parameters in DB
      /users?country="United Kingdom" - will return all users from the UK, note fields with spaces must have quotes
      /users?firstName=John - will return all johns
      
    PATCH /users/{userID}   - edits provided user fields for specified user in DB
      /users/3ee67cd8-8ff4-387a-b765-be1a46fd1bf9
      {
        "nickname": "KingSmithy"  - will update users nickname
      }
      
    DELETE /users/{userID}   - deletes user records in DB
      /users/3ee67cd8-8ff4-387a-b765-be1a46fd1bf9 -will delete user
      
    GET /__health    - checks whether application is able to take requests 

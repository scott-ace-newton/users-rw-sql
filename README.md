# users-rw-sql

## Introduction
Users Rw Sql is a RESTful API allowing for CRUD operations on User data

## Installation
Install sql and create schemas and download source code

        brew install mysql@5.6
        brew services start mysql@5.6
        
        mysql --host=localhost --user=root
        
        CREATE DATABASE test;
        CREATE DATABASE dev;

        go get -u github.com/scott-ace-newton/users-rw-sql
        cd $GOPATH/src/github.com/scott-ace-newton/users-rw-sql

## Running locally

1. Run the tests and install the binary:

        go test -v -race -cover
        go install

2. Run the binary (using the `help` flag to see the available optional arguments):

        $GOPATH/bin/users-rw-sql --sqlCredentials=root: --sqlDSN=dev --queueURL=/dev/nul

## Service endpoints

    PUT      `/users`           - adds user records to DB
      {
        "firstName": "John",
        "lastName": "Smith",
        "emailAddress": "JohnSmith@gmail.com",
        "password": "password1",
        "nickname": "smithy12345",
        "country": "UK"
      }  
    GET      `/users`           - returns user records matching parameters in DB
      /users?country=UK - will return all users from the UK
    PATCH    `/users/{userID}`  - edits provided user fields for specified user in DB
      /users/3ee67cd8-8ff4-387a-b765-be1a46fd1bf9
      {
        "nickname": "KingSmithy"  - will update users nickname
      }
    DELETE   `/users/{userID}`  - deletes user records in DB
      /users/3ee67cd8-8ff4-387a-b765-be1a46fd1bf9 -will delete user
    GET      `/__health`        - checks whether application is able to take requests 

FROM golang:1.11-alpine As builder

COPY . $GOPATH/src/github.com/scott-ace-newton/users-rw-sql

RUN cd ..

RUN go mod download

WORKDIR $GOPATH/src/github.com/scott-ace-newton/users-rw-sql

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/users-rw-sql

FROM scratch

# Copy our static executable from the builder
COPY --from=builder /go/bin/users-rw-sql /bin/users-rw-sql

EXPOSE 8080

ENTRYPOINT ["/bin/users-rw-sql"]

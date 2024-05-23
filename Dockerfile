# Start with the official Golang image
FROM golang:1.22-alpine

# Set the working directory inside the container
WORKDIR /app

# Copy the Go modules manifests and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the entire project
COPY . .

# Build the Go app
RUN go build -o main .

# Expose the port the app runs on
EXPOSE 8080

# Run the Go app
CMD ["./main"]

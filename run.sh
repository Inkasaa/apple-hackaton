#!/bin/bash
# Run script for Öfvergårds server
# This script ensures the server runs from the correct directory

cd "$(dirname "$0")/server"
go build -o ofvergards-backend . && ./ofvergards-backend

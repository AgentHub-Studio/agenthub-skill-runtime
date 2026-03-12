#!/bin/bash
MAVEN_IMAGE="maven:3.9.12-amazoncorretto-25"

docker run --rm \
  -v "$(pwd)":/app \
  -v "$HOME/.m2":/root/.m2 \
  -w /app \
  ${MAVEN_IMAGE} \
  mvn "$@"

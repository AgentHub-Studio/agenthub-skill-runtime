MAVEN_IMAGE := maven:3.9.12-amazoncorretto-25
DOCKER_RUN := docker run --rm -v $(PWD):/app -v $(HOME)/.m2:/root/.m2 -w /app $(MAVEN_IMAGE)

.PHONY: test compile package clean

test:
	$(DOCKER_RUN) mvn test

compile:
	$(DOCKER_RUN) mvn compile

package:
	$(DOCKER_RUN) mvn package -DskipTests

clean:
	$(DOCKER_RUN) mvn clean

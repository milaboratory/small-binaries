# Docker distribution of the package: real java that prints the same
# report as the fake java run environment of the binary distribution.
FROM eclipse-temurin:21-jdk

WORKDIR /app

COPY ./ /app

RUN javac Report.java

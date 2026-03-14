FROM ghcr.io/graalvm/jdk-community:21

RUN microdnf install -y fontconfig freetype dejavu-sans-fonts

COPY target/agenthub-skill-runtime.jar /opt/app.jar

ENV JAVA_OPTS -server \
    -Xms512M \
    -Xmx2G \
    -XX:+UseG1GC \
    -XX:MaxGCPauseMillis=200 \
    -Duser.timezone=Brazil/East \
    -Duser.language=pt \
    -Duser.country=BR \
    -Djava.net.preferIPv4Stack=true \
    -Djava.awt.headless=true

EXPOSE 8082

ENTRYPOINT exec java $JAVA_OPTS -jar /opt/app.jar

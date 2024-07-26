# Принтит время бездействия пользователя
apk add xprintidle

# Мониторинг
while true; do
    IDLE_TIME=$(xprintidle)
    TRASHHOLD=2400000 # 40 минут
    if [ "$IDLE_TIME" -ge "$TRASHHOLD" ]; then
        echo "No user activity detected for $((IDLE_TIME / 60000)) minutes. Stopping container."
        kill 1
    fi
    sleep 60
done

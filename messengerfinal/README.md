# Messenger

Локальный мессенджер для Windows. Go-сервер отдаёт веб-интерфейс и хранит аккаунты и сообщения в PostgreSQL.

## Запуск на Windows

Распакуйте весь проект. В папке `messengerfinal` откройте `Start-Messenger.cmd` двойным щелчком. При первом запуске PostgreSQL создаст пустую базу `messenger` и локальный файл `.env`. Это может занять около двух минут. Когда сервер запустится, откройте [http://localhost:9090](http://localhost:9090) и зарегистрируйте аккаунт.

Для раздельного запуска откройте `Start-PostgreSQL.exe`, затем `Start-Messenger.exe`. Оставьте окно мессенджера открытым во время работы. Чтобы остановить PostgreSQL, используйте `Stop-PostgreSQL.exe`.

При повторном запуске PostgreSQL использует существующую базу. Каталог `data`, настройки `.env` и журналы создаются локально и исключены из Git.

## Сборка из исходников

Из папки `messengerfinal` выполните в `cmd` с Go в PATH:

```cmd
go build -o messenger-postgres.exe ./cmd
go build -o Start-PostgreSQL.exe -ldflags "-H=windowsgui" ./tools/postgres-launcher
```

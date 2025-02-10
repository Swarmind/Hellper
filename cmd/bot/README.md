# Package: main

### Imports:
- hellper/internal/ai
- hellper/internal/database
- hellper/internal/telegram
- log
- os
- github.com/joho/godotenv

### External data, input sources:
- .env file for environment variables like DB_CONNECTION and BOT_TOKEN

### TODOs:
- None found

### Summary:
The main function initializes the application by loading environment variables from the .env file. It then creates instances of the database, AI, and Telegram services, using the loaded environment variables and the database service. Finally, it starts the Telegram service, which can be called as a non-blocking goroutine if needed.

The loadEnv function is used to retrieve environment variables from the .env file. It takes a key as input and returns the corresponding value. If the value is empty, it logs an error and exits the program.

```
bot/
├── main.go
└── cmd/
    └── bot/
        └── main.go
```

The code in the main.go file initializes the application by loading environment variables from the .env file. It then creates instances of the database, AI, and Telegram services, using the loaded environment variables and the database service. Finally, it starts the Telegram service, which can be called as a non-blocking goroutine if needed.

The loadEnv function is used to retrieve environment variables from the .env file. It takes a key as input and returns the corresponding value. If the value is empty, it logs an error and exits the program.

The code in the cmd/bot/main.go file is responsible for launching the bot. It takes the following arguments:

- -env: Path to the .env file containing environment variables.
- -debug: Enable debug mode.

The bot can be launched in the following ways:

1. Using the -env flag to specify the path to the .env file:

```
go run cmd/bot/main.go -env .env
```

2. Using the -debug flag to enable debug mode:

```
go run cmd/bot/main.go -debug
```

3. Using both flags:

```
go run cmd/bot/main.go -env .env -debug
```

The bot will use the environment variables from the .env file to connect to the database, initialize the AI service, and start the Telegram service.


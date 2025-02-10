# cmd/bot/main.go  
## Package: main  
  
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
  

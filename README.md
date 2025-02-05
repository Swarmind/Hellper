# Hellper
Telegram bot for OpenAI endpoints

# Setup
 - put bot token and modify db connection if needed in the `.env`
 - run database (there are postgres set up in compose.yml) `docker compose up -d`
 - run bot `go run cmd/bot/main.go`
 - go to database sql console or gui viewver and just create rows in auth_methods table
 - create rows in endpoints table - make sure to create a proper reference to auth_methods row id

## About auth methods
In our case we can have single access control point for different OpenAI endpoints, so the singlt access token can be used to access multiple endpoints.
Every endpoint can have the same reference to auth_methods row, e.g. http://swarmind-url/lai/federated and http://swarmind-url/lai/weights is related to auth_methods row with id 1;
https://openai-url is related to auth_methods row with id 2, so the OpenAI and Swarmind tokens can be used for different endpoints conveniently

# Package: database

### Imports:
- "database/sql"
- "_ "github.com/lib/pq"

### External Data, Input Sources:
- connectionString (string)

### TODOs:
- None

### Summary:
The database package provides a Handler struct that manages the database connection. The NewHandler function takes a connection string as input and returns a new Handler instance along with an error. It opens a connection to the database using the provided connection string and the "postgres" driver. If an error occurs during the connection process, it returns nil and the error. Otherwise, it creates a new Handler instance with the opened database connection and returns it.

File structure:
- init.go
- internal/database/init.go


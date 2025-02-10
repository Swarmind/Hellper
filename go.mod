module hellper

go 1.23.5

require (
	github.com/go-telegram/bot v1.13.3
	github.com/joho/godotenv v1.5.1
	github.com/lib/pq v1.10.9
	github.com/tmc/langchaingo v0.1.13-pre.1
)

require (
	github.com/dlclark/regexp2 v1.10.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/pkoukk/tiktoken-go v0.1.6 // indirect
)

replace github.com/tmc/langchaingo v0.1.13-pre.1 => github.com/Swarmind/langchaingo v0.0.1

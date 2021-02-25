# How to run

1. Realtime

- Go to realtime dir `cd realtime`
- Rename **.env.example** to **.env**, update `ABLY_KEY` with your own key 
- Realtime channel pub-sub

    ```go run presence.go constants.go```


- Realtime channel presence

    ```go run pub-sub.go constants.go```
    
2. Rest

- Go to rest dir `cd rest`     
- Rename **.env.example** to **.env**, update `ABLY_KEY` with your own key 
- Rest channel publish

    ```go run publish.go constants.go utils.go```

- Rest channel presence

    ```go run presence.go constants.go utils.go```

- Rest channel message history

    ```go run history.go constants.go utils.go```

- Rest application stats

    ```go run stats.go utils.go constants.go```

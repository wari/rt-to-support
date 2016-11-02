# rt-to-support

A simple utility hacked up in a day to download all the attachments from RT (Request Tracker) by Best Practical.

A guest login is automatically provided, while you can override by creating a file `rt.toml`, here's an example config:

```
rt = "https://rt.example.com"
user = "username"
pass = "password"
```

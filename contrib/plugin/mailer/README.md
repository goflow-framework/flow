# Mailer plugin (skeleton)

This package provides a small mailer plugin interface and a simple SMTP
adapter. It's intended as an example and a starting point for official
contrib plugins.

Usage
-----

```go
import (
  "github.com/undiegomejia/flow/contrib/plugin/mailer"
)

m := mailer.NewSMTPAdapter("smtp.example.com:587", "user", "pass")
_ = m.Send("bob@example.com", "hello", "body")
```

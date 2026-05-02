# Challenge: The Silent Write
**Difficulty:** Intro  
**Stack:** Node.js · Express · JSON storage

---

## The Symptom

> Creating a user returns `201 Created`, but after a server restart the user is missing.

---

## Your Environment

The server is already running. You have access to:
- This terminal (bash)
- All source files in `/app`
- `curl` for making HTTP requests
- `jq` for formatting JSON output

---

## Useful Commands

```bash
# Check the server is running
curl http://localhost:3000/health

# Create a user
curl -X POST http://localhost:3000/users \
  -H "Content-Type: application/json" \
  -d '{"name": "Jane Doe", "email": "jane@example.com"}' | jq

# Get all users
curl http://localhost:3000/users | jq

# Restart the server (simulate a production restart)
npm restart

# Browse the source files
ls /app
cat /app/src/routes/users.js   # example
```

---

## Submitting

When you've found and fixed the bug, run:
```bash
curl -X POST http://localhost:3000/users \
  -d '{"name": "Verify User", "email": "verify@example.com"}' \
  -H "Content-Type: application/json"

# Then restart
npm restart

# Then verify persistence
curl http://localhost:3000/users | jq
```

If your fix is correct, the user will still be there after restart.

---

*Use the AI assistant (top right) if you're stuck. It won't give you the answer but it will help you think.*
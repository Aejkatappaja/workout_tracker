# End-to-end flow of the browser UI (cookie session + HTMX), run from the shell:
#   hurl --test web.hurl
# Targets http://localhost:8080. Hurl keeps the session cookie across requests.
# Re-runnable: register is not asserted (409 on repeat is fine), login re-issues the cookie.

# Anonymous dashboard redirects to the login page (checked before any cookie is set)
GET http://localhost:8080/app
HTTP 303
[Asserts]
header "Location" == "/login"

# Register (status not asserted so the file re-runs without a DB reset)
POST http://localhost:8080/register
[FormParams]
username: e2e_web
email: e2e_web@example.com
password: whiterabbit

# Login -> 303 to /app and a session cookie
POST http://localhost:8080/login
[FormParams]
username: e2e_web
password: whiterabbit
HTTP 303
[Asserts]
header "Location" == "/app"
cookie "session" exists

# Dashboard now renders
GET http://localhost:8080/app
HTTP 200
[Asserts]
body contains "ls ~/workouts"

# New-workout form
GET http://localhost:8080/app/workouts/new
HTTP 200
[Asserts]
body contains "add exercise"

# Create a workout -> 200 + HX-Redirect to the new workout; capture its path
POST http://localhost:8080/app/workouts
[FormParams]
title: e2e push day
description: upper body
duration_minutes: 60
calories_burned: 200
entry_exercise: bench press
entry_sets: 3
entry_reps: 10
entry_duration:
entry_weight: 135.5
entry_notes: warmup
HTTP 200
[Asserts]
header "HX-Redirect" matches "^/app/workouts/[0-9]+$"
[Captures]
workout_path: header "HX-Redirect"

# Detail page shows the workout and its exercise
GET http://localhost:8080{{workout_path}}
HTTP 200
[Asserts]
body contains "e2e push day"
body contains "bench press"

# Validation: an exercise with neither reps nor duration is rejected inline (200, no redirect)
POST http://localhost:8080/app/workouts
[FormParams]
title: bad workout
duration_minutes: 30
entry_exercise: broken
entry_sets: 3
entry_reps:
entry_duration:
entry_weight:
entry_notes:
HTTP 200
[Asserts]
body contains "exactly one of reps or duration"
header "HX-Redirect" not exists

# Delete the created workout -> HX-Redirect back to the dashboard
DELETE http://localhost:8080{{workout_path}}
HTTP 200
[Asserts]
header "HX-Redirect" == "/app"

# It is gone now
GET http://localhost:8080{{workout_path}}
HTTP 404

# Logout clears the session and redirects to login
POST http://localhost:8080/logout
HTTP 303
[Asserts]
header "Location" == "/login"

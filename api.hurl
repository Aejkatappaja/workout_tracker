# Full API flow with assertions. Run in your shell:
#   hurl --test api.hurl
# Targets http://localhost:8080; sed the host if you run elsewhere.
# Re-runnable: register is not asserted (409 on repeat is fine), login re-issues a token.

# Health
GET http://localhost:8080/health
HTTP 200

# Register neo (status not asserted so the file re-runs without a DB reset)
POST http://localhost:8080/users
{
  "username": "neo",
  "email": "neo@matrix.io",
  "password": "whiterabbit"
}

# Register trinity
POST http://localhost:8080/users
{
  "username": "trinity",
  "email": "trinity@matrix.io",
  "password": "redpill123"
}

# Login neo -> capture bearer token
POST http://localhost:8080/tokens/authentication
{
  "username": "neo",
  "password": "whiterabbit"
}
HTTP 201
[Captures]
token: jsonpath "$.auth_token.token"

# Login trinity -> capture a second token (for the IDOR checks)
POST http://localhost:8080/tokens/authentication
{
  "username": "trinity",
  "password": "redpill123"
}
HTTP 201
[Captures]
tokenTrinity: jsonpath "$.auth_token.token"

# Login with a wrong password -> 401 (not 500)
POST http://localhost:8080/tokens/authentication
{
  "username": "neo",
  "password": "wrong"
}
HTTP 401

# Login unknown user -> 401 (no username-enumeration oracle)
POST http://localhost:8080/tokens/authentication
{
  "username": "ghost",
  "password": "whatever"
}
HTTP 401

# Create a workout (owner is derived from the token) -> capture its id
POST http://localhost:8080/workouts
Authorization: Bearer {{token}}
{
  "title": "push day",
  "description": "upper body",
  "duration_minutes": 60,
  "calories_burned": 200,
  "entries": [
    { "exercise_name": "bench press", "sets": 3, "reps": 10, "weight": 135.5, "order_index": 1 },
    { "exercise_name": "plank", "sets": 3, "duration_seconds": 60, "order_index": 2 }
  ]
}
HTTP 201
[Captures]
wid: jsonpath "$.workout.id"

# List my workouts -> 200, contains the one just created
GET http://localhost:8080/workouts
Authorization: Bearer {{token}}
HTTP 200
[Asserts]
jsonpath "$.workouts" count >= 1

# Owner reads it back -> 200
GET http://localhost:8080/workouts/{{wid}}
Authorization: Bearer {{token}}
HTTP 200
[Asserts]
jsonpath "$.workout.title" == "push day"
jsonpath "$.workout.entries" count == 2

# No token -> 401
GET http://localhost:8080/workouts/{{wid}}
HTTP 401

# Another user (IDOR) -> 403
GET http://localhost:8080/workouts/{{wid}}
Authorization: Bearer {{tokenTrinity}}
HTTP 403

# Missing workout -> 404
GET http://localhost:8080/workouts/99999999
Authorization: Bearer {{token}}
HTTP 404

# Non-numeric id -> 400
GET http://localhost:8080/workouts/abc
Authorization: Bearer {{token}}
HTTP 400

# Update by a non-owner -> 403
PUT http://localhost:8080/workouts/{{wid}}
Authorization: Bearer {{tokenTrinity}}
{
  "title": "hacked"
}
HTTP 403

# Update by the owner -> 200
PUT http://localhost:8080/workouts/{{wid}}
Authorization: Bearer {{token}}
{
  "title": "pull day",
  "entries": [
    { "exercise_name": "barbell row", "sets": 4, "reps": 8, "order_index": 1 }
  ]
}
HTTP 200

# Delete by a non-owner -> 403
DELETE http://localhost:8080/workouts/{{wid}}
Authorization: Bearer {{tokenTrinity}}
HTTP 403

# Delete by the owner -> 204
DELETE http://localhost:8080/workouts/{{wid}}
Authorization: Bearer {{token}}
HTTP 204

# Gone now -> 404
GET http://localhost:8080/workouts/{{wid}}
Authorization: Bearer {{token}}
HTTP 404

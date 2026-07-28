# SuIM File Service

The file service stores metadata in MySQL and file bytes in a private MinIO bucket. Clients request short-lived signed URLs through the API Gateway and transfer bytes directly to MinIO.

## HTTP API

All endpoints require the existing `Authorization: Bearer <token>` header.

| Method | Path | Purpose |
| --- | --- | --- |
| `POST` | `/api/v1/files/initiate` | Validate metadata and issue a signed PUT URL |
| `POST` | `/api/v1/files/{id}/complete` | Verify size, MIME and SHA-256, then publish the file |
| `GET` | `/api/v1/files/{id}` | Read authorized file metadata |
| `GET` | `/api/v1/files/{id}/download` | Issue a five-minute signed download URL |
| `DELETE` | `/api/v1/files/{id}` | Delete an unbound file owned by the caller |

The Gateway calls the internal `BindFile` RPC when it accepts a file message. A binding grants access to users who own the corresponding conversation row.

## Lifecycle

- Upload signatures expire after 15 minutes.
- Pending or uploaded-but-unbound files are removed after 24 hours.
- A conversation binding extends both the file and binding expiry to 180 days.
- Cleanup runs hourly in batches and removes both the MinIO object and metadata bindings.

## Limits

- Maximum file size: 100 MiB.
- Per-user quota: 10 GiB.
- Executables, shell scripts, HTML and SVG are rejected.
- Values are configurable through the `FILE_*` environment variables in `deploy/docker-compose.yml`.

For browser uploads, `MINIO_PUBLIC_ENDPOINT` must be reachable from the browser, not merely from the Docker network.

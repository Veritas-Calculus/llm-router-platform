# JWT Asymmetric Signing + Key Rotation Runbook

The platform's access + refresh tokens default to HS256 with a single static `JWT_SECRET`. Anyone with read access to that secret (env file, container image, backup, log accident) can forge tokens for any user — see audit §5.4 M4.

This runbook walks through:

1. Migrating from HS256 to RS256 (or Ed25519) with zero downtime.
2. Rotating the signing key on a schedule.

The implementation lives in `pkg/jwtsign` and `internal/api/middleware/jwt_signer.go`. Existing HS256 deployments need **no** config change — the signer falls back to HS256 + `JWT.Secret` when no explicit algorithm is configured.

## Why asymmetric

| Concern | HS256 (current) | RS256 / EdDSA (target) |
|---|---|---|
| Secret compromise | Forge any token | Forge requires private key, only on signer |
| Verifier deployment | Must hold the secret | Public key sufficient |
| Cross-service verification | Each service holds secret | JWKS endpoint distributes public key |
| Key rotation | Drop all sessions | Multi-key verify, zero downtime |

RS256 is the safe default (universally supported). EdDSA (Ed25519) is smaller and faster but a small subset of consumers may not support it yet.

## One-time migration (HS256 → RS256)

### 1. Generate the keypair

```bash
# Pick a stable identifier; "v1" works for the first key.
KID=v1
openssl genrsa -out jwt-${KID}.pem 2048
openssl rsa  -in jwt-${KID}.pem -pubout -out jwt-${KID}.pub
```

Use a separate keypair for each environment (dev/staging/prod). Store the private key in your secrets manager.

### 2. Configure the server

Set these environment variables (or their `config.yaml` equivalents):

```env
JWT_ALGORITHM=RS256
JWT_KEY_ID=v1
JWT_PRIVATE_KEY_PEM=<paste contents of jwt-v1.pem>
JWT_PUBLIC_KEY_PEM=<paste contents of jwt-v1.pub>    # optional; derived from private if omitted
```

Keep `JWT_SECRET` set during the rollout — it stays available as a fallback if `BuildJWTSigner` errors at boot.

### 3. Roll out the signer

Deploy. The middleware logs:

```
INFO  jwt signer initialized   algorithm=RS256 kid=v1
```

Existing HS256-signed tokens issued before the deploy will not verify under the new signer. Two options:

- **Forced re-login**: simplest. Pre-deploy, invalidate every user's `TokensInvalidatedAt` so the next access-token refresh kicks them out cleanly.
- **Dual-stack**: leave `JWT_SECRET` populated AND set `JWT_ALGORITHM=RS256`. The middleware tries the signer first; on failure it falls back to the legacy HS256 path. After the maximum access-token lifetime (default 1 hour) has elapsed since the deploy, clear `JWT_SECRET`.

## Periodic key rotation

### 1. Generate a new keypair

```bash
KID=v2
openssl genrsa -out jwt-${KID}.pem 2048
openssl rsa  -in jwt-${KID}.pem -pubout -out jwt-${KID}.pub
```

### 2. Configure the server with v2 current + v1 previous

```env
JWT_KEY_ID=v2
JWT_PRIVATE_KEY_PEM=<contents of jwt-v2.pem>
JWT_PUBLIC_KEY_PEM=<contents of jwt-v2.pub>
JWT_PREVIOUS_KEYS_PEM_V1=<contents of jwt-v1.pub>    # public key only
```

The signer mints new tokens with `kid=v2` and still accepts incoming tokens with `kid=v1` until they expire.

### 3. Retire v1 after the refresh-token lifetime

Default refresh tokens expire after 7 days. After that window, remove the `JWT_PREVIOUS_KEYS_PEM_V1` entry and delete the v1 private key from your secrets manager.

## JWKS endpoint (future work)

Downstream services that want to verify tokens server-side should fetch the public key from a `/.well-known/jwks.json` endpoint instead of receiving the PEM out-of-band. The signer exposes `CurrentKID()` and `VerifyPublicKey(kid)` so wiring this endpoint is a small additional change; not yet shipped.

# API local · v0.1

Contrato del cliente local, no de un servicio remoto. Todas las respuestas JSON; errores `{ "error": "mensaje es-ES" }`.

El servicio escucha exclusivamente en loopback numérico (por defecto `127.0.0.1`; admite `::1`), valida Host/Origin y exige `X-Elciber-Token` en **toda** `/api/`. El HTML inyecta la sesión aleatoria en `<meta name="elciber-token" content="__ELCIBER_TOKEN__">`. Mutaciones: `Content-Type: application/json`. No hay CORS ni telemetría.

- `GET /api/state` → `{version, rooms: Room[], engine: Engine}`.
- `POST /api/rooms` con `{name, game, rendezvous, rendezvousKey}` → Room pública (sin secreto).
- `POST /api/rooms/import` con `{invite}` → Room pública. Guardar no conecta. Conflicto de identidad rechaza; nunca sobrescribe secretos.
- `POST /api/rooms/{id}/invite` con `{}` → `{invite}`. La invitación es un secreto compartido, NO de un solo uso ni cifrada.
- `DELETE /api/rooms/{id}` con `{}` → `{ok:true}`. No permite borrar una sala activa.
- `POST /api/connect` con `{roomId, acknowledge:true}` → `{ok:true}`. Requiere confirmar confianza en invitados y destino. Una conexión activa por cliente. En inspección solo permite destinos loopback numéricos.
- `POST /api/disconnect` con `{}` → `{ok:true}`. Idempotente.
- `POST /api/quit` con `{}` → `{ok:true}` y cierre del proceso local con su motor. Exige la misma sesión y origen; las salas se conservan.

`Room`: `{id, name, game, rendezvous, rendezvousKey, createdAt}`. `rendezvous` es una URL `tcp://host:puerto` o `udp://host:puerto`; puede quedar vacío para guardar la sala, pero VPN exige un destino explícito. `rendezvousKey` es la clave pública base64 de 32 bytes del nodo de encuentro (no un secreto); puede omitirse para guardar o inspección, pero es obligatoria para conectar VPN, como `peer_public_key` de EasyTier. Evita confiar únicamente en una dirección frente a un nodo de otra red. **Límite de EasyTier 2.6.4:** un nodo que demuestre conocer el secreto de esta sala se acepta aunque su clave no coincida; no hay pin estricto frente a otros miembros de la sala. Se valida al crear/importar y se vuelve a mostrar antes de conectar. No hay nodo público preconfigurado.

`Engine`: `{available, mode, status, roomId, message, version, peers, virtualIP}`.

- `mode`: `inspection` (no crea adaptadores/rutas) o `vpn` (solo si se inició con `--enable-vpn`).
- `status`: `stopped`, `starting`, `running`, `error`.
- `peers`: `[{id, hostname, ipv4, latencyMs, connection}]`, exclusivamente datos reales de EasyTier. `latencyMs` admite null; `connection` puede ser `direct`, `relay` o `unknown`.
- `virtualIP`: vacío si no hay evidencia del motor; nunca inventar IP/latencia/presencia. `running` significa proceso y RPC operativos, no garantiza LAN funcional ni amigos conectados.

La UI muestra errores y ausencia de motor claramente. Crear/guardar/invitar funciona sin instalar EasyTier. No se lanza ningún proceso del motor al arrancar la aplicación; puede abrirse el navegador. La API no acepta rutas ejecutables, argumentos libres, DNS, rutas, proxies, shell, privilegios ni opciones del motor.

# Historial de cambios

El estado de una versión describe lo entregado, no la aspiración del producto.

## Trabajo posterior de la alpha · 2026-09-10

- Candidato NSIS de instalador único, helper Go de descarga verificada, arranque gráfico y cierre autenticado. Compilado y probado localmente; instalación Windows pendiente.
- Documento de estado y reanudación, mapa de fuentes y protocolo de continuidad en Git.
- Dirección de producto fijada: gratuito, sin servidor de pago, P2P preferente con encuentro y relay comunitarios gratuitos. Son objetivos pendientes de integración, no funcionalidades ya probadas.

## 0.1.0-alpha.1 · 2026-09-10

### Añadido

- Cliente local Go e interfaz es-ES embebida, sin Electron.
- Salas persistentes, invitaciones versionadas, importación sin autoconexión y eliminación local explícita.
- Adaptador de proceso para EasyTier 2.6.4 con inspección sin TUN y modo VPN explícito.
- Control local con sesión efímera, comprobación de origen y validación estricta de entradas.
- Descargadores opcionales con SHA-256 fijado; sin autoactualización, servicios ni instalación de drivers.
- Pruebas de navegador, integración loopback real y configuración de CI Windows/Linux, conservada como plantilla sin activar.
- README visual, documentación de seguridad y hoja de ruta.

### Límites

No es una release estable ni un instalador. La publicación de código no acredita despliegue, firma, pruebas de TUN en Windows, partida entre redes distintas, descubrimiento automático, compatibilidad por juego o rendimiento prolongado.

Ingeniería: **OTACON Astra**.

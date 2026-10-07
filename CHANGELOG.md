# Historial de cambios

El estado de una versión describe lo entregado, no la aspiración del producto.

## En desarrollo · 2026-10-07

- La política VPN deja de bloquear P2P: habilita conexión directa y TCP/UDP/symmetric hole punching.
- VPN recupera el conjunto STUN integrado de EasyTier 2.6.4 y permite relay de datos como respaldo; inspección sigue aislada sin STUN, P2P ni relay de datos.
- Se mantienen `private_mode`, UPnP desactivado y relays KCP/QUIC desactivados para no ampliar más superficie de la necesaria.
- Añadidas pruebas de regresión que fijan la separación entre inspección y VPN. Sigue pendiente validar dos redes reales, el nodo de encuentro y la ruta directo/relay.
- La UI deja de inferir una ruta directa por `tunnel_proto`: usa el `cost` de EasyTier 2.6.4 (`p2p` o `relay(n)`), con regresión para relay sobre transporte TCP.


## Colaboración en EspacioKoop · 2026-09-10

- Repositorio transferido a `EspacioKoop/elciber`, conservando identidad e historial.
- Acceso compartido comprobado para `VaroTv7` y `eGurucharri`; enlaces, módulo Go y punto de reanudación actualizados.
- La transferencia no activa CI, despliega servicios ni completa la conectividad o el instalador pendientes.

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

# Arquitectura

## Una interfaz propia sobre un motor de red existente

El Ciber implementa la experiencia de salas, invitaciones, persistencia y control local. EasyTier implementa el transporte de red. No reinventamos criptografía, NAT traversal ni controladores.

```text
Navegador del equipo
       │ HTTP loopback + sesión efímera + validación de origen
       ▼
Cliente Go · API local · salas guardadas
       │ configuración privada + ejecución sin shell
       ▼
EasyTier core ← CLI de estado limitada a loopback
       │ modo VPN explícito
       ▼
Adaptador virtual ─── nodo explícito autenticado ─── amigos
                      (alpha: P2P automático desactivado)
```

## Decisiones de la primera versión

- **Go y biblioteca estándar:** servidor, almacenamiento y supervisión de procesos; sin dependencias de ejecución de Go externas.
- **HTML, CSS y JavaScript nativos:** embebidos en el ejecutable; navegador del sistema, sin Electron, React, servidores web externos, CDN o fuentes descargadas.
- **EasyTier externo y fijado:** actualización deliberada, no descarga automática durante el arranque. Sus binarios no se incluyen en Git.
- **Salas locales, sin cuenta central:** crear y compartir no necesita servidores de El Ciber. Para unir redes físicas distintas sí hace falta un destino alcanzable; no existe un relay operado por este proyecto.
- **Una sala activa:** simplifica el ciclo de vida, las IP y el aislamiento de configuración. Se pueden guardar varias salas sin abrir conexiones.
- **Modo de inspección predeterminado:** permite revisar UI, persistencia y control del proceso sin crear TUN. No sirve para jugar por LAN virtual.
- **No al tráfico de salida general:** no se configura exit node, proxy de subred doméstica ni DNS del sistema. No es una VPN de navegación.

## Diseño objetivo · no implementado aún

Encuentro con nodos comunitarios gratuitos y autenticados; conexión directa entre amigos como primera opción y relay gratuito solo cuando la directa no sea viable. No exige un servidor de pago ni configurar nodos, IPs o claves técnicas al jugador. El servicio de encuentro no transporta el tráfico de juego cuando existe una ruta P2P directa. Se deben validar confianza, disponibilidad y latencia de los nodos, sin rebajar autenticación por conveniencia.

El instalador web ya se compila y prepara el motor automáticamente, pero eso no implementa este diseño. La referencia para retomar es [PROJECT_STATUS.md](../PROJECT_STATUS.md).

## Conectividad real frente a experiencia de sala

La sala representa una identidad de red y material de acceso compartido. No es un servidor de juego ni un lobby dentro del juego. La aplicación no decide quién hospeda la partida.

En modo VPN, los clientes necesitan un nodo de encuentro compatible con EasyTier 2.6.4 y su clave pública verificada. Esta primera versión no administra ni despliega el nodo. La conectividad depende de rutas, firewall y disponibilidad externa. En esta alpha se conecta al destino explícito; la búsqueda automática P2P, STUN y hole punching siguen desactivados también en VPN. No se acredita todavía una malla P2P automática entre los amigos; normalmente se necesitará relay a través del nodo.

Un adaptador TUN transporta IP; no replica automáticamente todo Ethernet. Algunos juegos permiten introducir una IP; otros usan broadcast, multicast, IPX o mecanismos propios. **El soporte del motor no es evidencia de compatibilidad de El Ciber.**

## Estado y fallos

Guardar, invitar e importar no requieren motor. El backend supervisa únicamente el proceso que inicia, y la interfaz distingue ausencia del motor, parado, arranque, activo y error. La presencia y la IP se muestran solo si llegan del motor. Un proceso activo no demuestra que una partida sea alcanzable.

El cierre normal debe detener el motor propio. Un apagado forzado puede impedir limpieza: esta versión no promete reconciliación de procesos y adaptadores tras un crash. El servicio local y la RPC no protegen frente a procesos maliciosos del mismo usuario.

## Evolución

Las prioridades son integrar encuentro gratuito y P2P preferente, retirar configuración técnica del flujo normal y separar privilegios de red de la UI. El candidato de instalador existe, pero faltan validación Windows, recuperación y partida real; después vendrán diagnóstico por juego y credenciales temporales. Véase [hoja de ruta](../ROADMAP.md).

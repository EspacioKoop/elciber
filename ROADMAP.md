# Hoja de ruta

**Dirección acordada:** gratuito, open source, instalar y jugar; conexión directa entre amigos como primera opción y relay comunitario gratuito de respaldo. El estado recuperable y los límites están en [PROJECT_STATUS.md](PROJECT_STATUS.md).

## Base técnica existente

- Salas locales, invitaciones y panel funcional.
- Control real de EasyTier 2.6.4, inspección sin TUN y controles de seguridad del servicio local.
- Pruebas locales, documentación y capturas reales.
- Instalador web Windows compilado, descarga automática del motor y cierre gráfico; pendiente instalación real.

## Criterio de producto · Instalar y jugar

Un único instalador prepara cliente y motor. El jugador crea una sala o acepta una invitación, sin instalar EasyTier aparte ni introducir nodos, IPs o claves técnicas. El objetivo no requiere alquilar un servidor ni que el jugador administre infraestructura.

**El prototipo actual no cumple aún este criterio.** La existencia del instalador no resuelve por sí sola la conexión automática.

## Prioridad 1 · Encuentro gratuito y P2P preferente

- Seleccionar y validar nodos comunitarios compatibles con el modo seguro y una distribución fiable de claves públicas.
- Integrar encuentro automático y conexión directa entre amigos. Activar NAT traversal de forma deliberada y probada; actualmente está desactivado también en VPN.
- Usar relay gratuito solo cuando no sea viable la conexión directa; explicar disponibilidad y límites sin garantías de servicio inventadas.
- Mostrar tipo de conexión y mediciones reales, sin estados de éxito simulados.
- Retirar los parámetros técnicos del flujo normal, manteniendo diagnóstico útil.

## Prioridad 2 · Distribución y seguridad

- Separar helper privilegiado e interfaz; minimizar solicitudes UAC y superficie elevada.
- Instalar/desinstalar en Windows 11 x64 real, preservando los datos del perfil.
- Implementar recuperación tras descarga interrumpida, destino parcial, actualización y rollback. El candidato actual rechaza instalaciones existentes.
- Evaluar firma/distribución verificable sin asumir gastos autorizados ni prometer ausencia de alertas de antivirus.

## Prioridad 3 · Primera partida real y rendimiento

- Probar dos PC Windows 11 en redes distintas con una partida sostenida.
- Verificar adaptador, IP virtual, firewall y tráfico TCP/UDP real.
- Probar P2P y relay por separado; medir latencia, jitter, pérdida y memoria/CPU del cliente, motor y navegador.
- Comparar con Hamachi/ZeroTier en las mismas condiciones antes de afirmar superioridad.
- Verificar desconexión, suspensión, corte de conexión, caída del nodo y recuperación.
- Registrar compatibilidad por juego; conexión por IP y descubrimiento LAN son criterios distintos.

**No está cerrado mientras falte una partida real de extremo a extremo.**

## Evolución posterior

- Invitaciones temporales, permisos, expulsión individual y rotación segura.
- Edición de salas sin sobrescritura silenciosa y bandeja del sistema.
- Conflictos de subred, MTU y diagnóstico claro de NAT.
- Linux con TUN real; evaluar macOS después.

## Fuera de alcance actual

Chat, voz, tienda, VPN de salida general a Internet, publicación de redes privadas y servidores de pago obligatorios. No se crean crons, agentes autónomos, infraestructura pública o servicios de sistema para avanzar esta hoja de ruta sin autorización específica.

# Automatización de pruebas · plantilla conservada

La configuración íntegra está en [ci.yml.example](ci.yml.example). Se conserva como fuente, **fuera de `.github/workflows/`**, y no activa GitHub Actions ni equivale a una ejecución aprobada.

La publicación inicial prioriza que el código, las capturas, las decisiones y el estado sean recuperables. La activación de CI queda pendiente de una autorización de publicación que cubra workflows; no se han cambiado credenciales ni eludido controles para activarla.

## Cobertura preparada

- Go: análisis estático, pruebas con detección de carreras y compilación en Windows/Linux.
- Análisis sintáctico del descargador PowerShell en Windows, sin ejecutarlo.
- Navegador Chromium real y motor EasyTier oficial en inspección loopback sobre Linux.
- Acciones fijadas por commit, permisos de contenido de solo lectura y límites de ejecución. Sin secretos de proyecto ni runners privados configurados por esta plantilla.

Los mismos tests pueden ejecutarse localmente siguiendo [VALIDATION.md](../VALIDATION.md). No se han eliminado scripts ni regresiones para publicar sin automatización activa.

Para activarla más adelante, una persona autorizada debe revisar el candidato, disponer del permiso de workflows, situar la plantilla como `.github/workflows/ci.yml` y comprobar las ejecuciones reales del commit. Esa activación no forma parte de este snapshot. No afirmar éxito de CI antes de leer sus resultados.

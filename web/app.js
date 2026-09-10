/* el ciber · Vanilla client. The local service is the source of truth. */
(() => {
  'use strict';

  const $ = (id) => document.getElementById(id);
  const token = document.querySelector('meta[name="elciber-token"]')?.content || '';
  const dialogs = [...document.querySelectorAll('dialog')];
  const roomNodes = new Map();
  const dialogOpeners = new WeakMap();
  const statusLabels = { stopped: 'Motor detenido', starting: 'Iniciando motor', running: 'Motor operativo', error: 'Error del motor' };
  let state = null;
  let selectedRoomId = null;
  let online = false;
  let pending = false;
  let stateRequest = null;
  let pollTimer = null;
  let toastTimer = null;
  let connectSnapshot = null;
  let deleteRoomId = null;
  let peerSignature = '';
  let copying = false;

  try { selectedRoomId = localStorage.getItem('selectedRoomId'); } catch { /* Storage may be disabled. */ }

  function text(id, value) {
    const node = $(id);
    const next = value == null ? '' : String(value);
    if (node.textContent !== next) node.textContent = next;
  }

  function element(tag, className, content) {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (content != null) node.textContent = String(content);
    return node;
  }

  function icon(name) {
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.classList.add('icon');
    svg.setAttribute('aria-hidden', 'true');
    const use = document.createElementNS('http://www.w3.org/2000/svg', 'use');
    use.setAttribute('href', `#i-${name}`);
    svg.append(use);
    return svg;
  }

  function persistSelection(id) {
    selectedRoomId = id || null;
    try {
      if (selectedRoomId) localStorage.setItem('selectedRoomId', selectedRoomId);
      else localStorage.removeItem('selectedRoomId');
    } catch { /* Selection still works in memory. Never persist invitations or the token. */ }
  }

  function currentRoom() {
    return state?.rooms.find((room) => room.id === selectedRoomId) || null;
  }

  function activeEngine() {
    const engine = state?.engine;
    return Boolean(engine && (['starting', 'running'].includes(engine.status) || (engine.roomId && engine.status === 'error')));
  }

  function roomOwnsEngine(room) {
    return Boolean(room && activeEngine() && state.engine.roomId === room.id);
  }

  function selectRoom(id, focus = false) {
    persistSelection(id);
    render();
    if (focus) $('workspace').focus({ preventScroll: true });
  }

  function notify(message, error = false) {
    clearTimeout(toastTimer);
    text('toast-message', message);
    $('toast').dataset.error = String(error);
    $('toast').querySelector('use').setAttribute('href', error ? '#i-info' : '#i-check');
    $('toast').hidden = false;
    toastTimer = setTimeout(() => { $('toast').hidden = true; }, error ? 10000 : 6500);
  }

  function showError(dialog, message = '') {
    const node = dialog.querySelector('.form-error');
    node.textContent = message;
    node.hidden = !message;
  }

  async function api(path, method = 'GET', body) {
    if (!token || token === '__ELCIBER_TOKEN__') {
      throw new Error('Abre el ciber desde su aplicación local. Esta página necesita una sesión válida; no funciona abriendo el archivo HTML directamente.');
    }
    const controller = new AbortController();
    const timeout = setTimeout(() => controller.abort(), method === 'GET' ? 12000 : 30000);
    const options = {
      method,
      headers: { 'X-Elciber-Token': token, Accept: 'application/json' },
      credentials: 'same-origin',
      cache: 'no-store',
      redirect: 'error',
      signal: controller.signal,
    };
    if (method !== 'GET') {
      options.headers['Content-Type'] = 'application/json';
      options.body = JSON.stringify(body ?? {});
    }
    try {
      const response = await fetch(`/api${path}`, options);
      const data = await response.json().catch(() => null);
      if (!response.ok) {
        throw new Error(typeof data?.error === 'string' ? data.error : `La aplicación local ha respondido con un error (${response.status}).`);
      }
      if (!data || typeof data !== 'object') throw new Error('La aplicación local ha enviado una respuesta que no se puede leer.');
      return data;
    } catch (error) {
      if (error.name === 'AbortError') throw new Error('La aplicación local no ha respondido a tiempo. Comprueba el estado antes de repetir la acción.');
      if (error instanceof TypeError) throw new Error('No se puede contactar con la aplicación local. Comprueba que el ciber sigue abierto y revisa el estado antes de repetir.');
      throw error;
    } finally {
      clearTimeout(timeout);
    }
  }

  async function refreshState(afterMutation = false) {
    // A mutation needs a read started AFTER it completed, never an older in-flight poll.
    if (stateRequest) {
      await stateRequest;
      if (!afterMutation) return online;
    }
    if (stateRequest) return stateRequest;
    stateRequest = (async () => {
      try {
        const next = await api('/state');
        if (!Array.isArray(next.rooms) || !next.engine || !next.rooms.every((room) => room && typeof room.id === 'string' && typeof room.name === 'string')) {
          throw new Error('El estado de la aplicación no tiene el formato esperado. Actualiza el ciber y vuelve a abrir su ventana.');
        }
        state = next;
        online = true;
        $('connection-error').hidden = true;
        if (selectedRoomId && !state.rooms.some((room) => room.id === selectedRoomId)) persistSelection(null);
        render();
        return true;
      } catch (error) {
        online = false;
        text('connection-error-text', `${error.message}${state ? ' Los datos visibles son los últimos recibidos, no un estado en directo.' : ''}`);
        $('connection-error').hidden = false;
        if (!state) text('sidebar-empty', 'No se han podido cargar las salas.');
        text('engine-status', 'Sin comunicación con la app');
        $('engine-status-dot').dataset.status = 'error';
        syncControls();
        return false;
      }
    })();
    try { return await stateRequest; } finally { stateRequest = null; }
  }

  async function poll() {
    clearTimeout(pollTimer);
    await refreshState();
    pollTimer = setTimeout(poll, 3000);
  }

  function isNumericLoopback(destination) {
    try {
      const host = new URL(destination).hostname;
      if (host === '[::1]' || host === '::1') return true;
      const octets = host.split('.');
      return octets.length === 4 && octets[0] === '127' && octets.every((part) => /^\d{1,3}$/.test(part) && Number(part) <= 255);
    } catch { return false; }
  }

  function connectionBlocker(room) {
    if (!room) return 'Selecciona una sala.';
    if (!online) return 'Espera a recuperar la comunicación con la aplicación local.';
    const engine = state.engine;
    if (!engine.available) return 'Falta EasyTier. Puedes guardar y compartir esta sala; consulta la guía para preparar el motor.';
    if (activeEngine()) return roomOwnsEngine(room) ? 'El motor ya está iniciado para esta sala.' : 'Hay un motor iniciado para otra sala. Detén esa sesión antes de cambiar de sala.';
    if (!['inspection', 'vpn'].includes(engine.mode)) return 'El modo del motor no es conocido. No se permite iniciar la conexión.';
    if (!room.rendezvous) return 'Esta sala no tiene destino. Puedes compartirla, pero para conectar tendrás que crear o guardar una sala con un destino explícito.';
    if (engine.mode === 'inspection' && !isNumericLoopback(room.rendezvous)) return 'En inspección solo se admiten destinos loopback numéricos (127.0.0.1 o ::1). Este destino no se iniciará en ese modo.';
    if (engine.mode === 'vpn' && !room.rendezvousKey) return 'Falta la clave pública del nodo. Pídesela a quien lo gestione y crea o guarda una sala que la incluya para conectar en modo VPN.';
    return '';
  }

  function renderRooms() {
    const rooms = state.rooms;
    const ids = new Set(rooms.map((room) => room.id));
    for (const [id, nodes] of roomNodes) {
      if (!ids.has(id)) { nodes.button.remove(); roomNodes.delete(id); }
    }
    // Keyed nodes preserve keyboard focus and scroll position across 3-second polls.
    rooms.forEach((room, index) => {
      let nodes = roomNodes.get(room.id);
      if (!nodes) {
        const button = element('button', 'room-nav-item');
        button.type = 'button';
        const copy = element('span', 'room-nav-copy');
        const name = element('span', 'room-nav-name');
        const game = element('span', 'room-nav-game');
        const dot = element('span', 'room-nav-active');
        dot.setAttribute('aria-hidden', 'true');
        copy.append(name, game);
        button.append(icon('room'), copy, dot);
        button.addEventListener('click', () => selectRoom(room.id));
        nodes = { button, name, game, dot };
        roomNodes.set(room.id, nodes);
      }
      if (nodes.name.textContent !== room.name) nodes.name.textContent = room.name;
      const game = room.game || 'Sin juego indicado';
      if (nodes.game.textContent !== game) nodes.game.textContent = game;
      nodes.dot.hidden = !roomOwnsEngine(room);
      nodes.button.setAttribute('aria-current', String(room.id === selectedRoomId));
      nodes.button.setAttribute('aria-label', `${room.name}. ${game}${roomOwnsEngine(room) ? '. Motor iniciado para esta sala' : ''}`);
      nodes.button.title = room.name;
      const existing = $('room-list').children[index];
      if (existing !== nodes.button) $('room-list').insertBefore(nodes.button, existing || null);
    });
    $('room-count').hidden = false;
    text('room-count', rooms.length);
    $('sidebar-empty').hidden = rooms.length > 0;
    text('sidebar-empty', 'Todavía no hay salas. La primera la ponéis vosotros.');
    text('empty-eyebrow', rooms.length ? 'ELIGE DÓNDE OS ENCONTRÁIS' : 'EMPEZAD POR AQUÍ');
    text('empty-title', rooms.length ? 'Otra partida empieza aquí.' : 'Una sala. Vuestra gente.');
    text('empty-description', rooms.length ? 'Selecciona una sala de la lista, crea otra o guarda una invitación. Nada se conecta sin que lo decidas tú.' : 'Crea una sala y comparte la invitación en privado. ¿Ya tienes una? Guárdala sin conectarte.');
  }

  function renderEngine() {
    const engine = state.engine;
    const inspection = engine.mode === 'inspection';
    text('engine-mode', inspection ? 'MODO INSPECCIÓN' : engine.mode === 'vpn' ? 'MODO VPN' : 'MODO DESCONOCIDO');
    text('engine-headline', inspection ? 'No crea red virtual' : engine.mode === 'vpn' ? 'Conexión bajo tu control' : 'Revisa el motor');
    text('engine-explanation', inspection ? 'Sirve para inspeccionar el motor sin crear adaptadores ni rutas. No conecta vuestros juegos por LAN.' : engine.mode === 'vpn' ? 'Este modo puede crear un adaptador y rutas de red. Solo se inicia después de tu confirmación.' : 'No se puede determinar qué hace este modo. La conexión está deshabilitada.');
    text('engine-status', statusLabels[engine.status] || 'Estado desconocido');
    $('engine-status-dot').dataset.status = engine.status;
    $('engine-install').hidden = Boolean(engine.available);
    text('engine-message', engine.message || '');
    $('engine-message').hidden = !engine.message;
    text('engine-version', engine.version ? `EasyTier · ${engine.version}` : '');
    $('engine-version').hidden = !engine.version;
    $('stop-engine').hidden = !activeEngine();
    document.querySelectorAll('.inspection-only').forEach((node) => { node.hidden = !inspection; });
  }

  function renderPeers(room) {
    const hasData = roomOwnsEngine(room) && state.engine.status === 'running';
    const peers = hasData && Array.isArray(state.engine.peers) ? state.engine.peers : [];
    const signature = JSON.stringify([room.id, hasData, peers]);
    text('peers-count', hasData ? `${peers.length} ${peers.length === 1 ? 'peer observado' : 'peers observados'}` : 'Sin datos');
    $('peers-empty').hidden = peers.length > 0;
    $('peer-list').hidden = peers.length === 0;
    $('peer-disclaimer').hidden = !hasData;
    text('peers-empty-title', hasData ? 'No hay peers confirmados por el motor' : 'Todavía no hay datos de conexión');
    text('peers-empty-description', hasData ? 'El proceso está operativo, pero no ha comunicado otros equipos. Esto no demuestra una LAN funcional.' : 'Solo se muestran equipos e IP que el motor haya confirmado.');
    if (signature === peerSignature) return;
    peerSignature = signature;
    const fragment = document.createDocumentFragment();
    for (const peer of peers) {
      const row = element('div', 'peer-item');
      const identity = element('div', 'peer-identity');
      identity.append(element('strong', '', peer.hostname || peer.id || 'Equipo sin nombre'));
      identity.append(element('span', '', peer.ipv4 || 'IP no comunicada'));
      const connection = element('div', 'peer-connection', ({ direct: 'Conexión directa', relay: 'Vía relay', unknown: 'Tipo no indicado' })[peer.connection] || 'Tipo no indicado');
      const latency = typeof peer.latencyMs === 'number' && Number.isFinite(peer.latencyMs) && peer.latencyMs >= 0 ? `${new Intl.NumberFormat('es-ES', { maximumFractionDigits: 1 }).format(peer.latencyMs)} ms` : 'Latencia no disponible';
      connection.append(element('span', '', latency));
      row.append(icon('room'), identity, connection);
      fragment.append(row);
    }
    $('peer-list').replaceChildren(fragment);
  }

  function actualIP(room) {
    return roomOwnsEngine(room) && state.engine.status === 'running' && state.engine.mode === 'vpn' && typeof state.engine.virtualIP === 'string' ? state.engine.virtualIP : '';
  }

  function renderRoom(room) {
    text('room-title', room.name);
    text('room-game', room.game || 'Sin juego indicado');
    const created = new Date(room.createdAt);
    text('room-created', Number.isNaN(created.getTime()) ? '' : `Guardada el ${new Intl.DateTimeFormat('es-ES', { day: 'numeric', month: 'short', year: 'numeric' }).format(created)}`);
    const own = roomOwnsEngine(room);
    const engine = state.engine;
    const status = own ? engine.status : 'stopped';
    const inspection = engine.mode === 'inspection';
    text('room-status', ({ stopped: 'Sin conectar', starting: 'Iniciando motor', running: 'Motor operativo', error: 'El motor necesita atención' })[status] || 'Estado desconocido');
    $('room-status-symbol').dataset.status = status;
    let description = 'La sala está guardada en este equipo. No hay ninguna conexión iniciada para esta sala.';
    if (status === 'starting') description = inspection ? 'El motor está arrancando en modo inspección. No crea red virtual.' : 'El motor está arrancando. Aún no hay evidencia de una conexión operativa.';
    if (status === 'running') description = inspection ? 'EasyTier responde en modo inspección. No crea red virtual ni conecta vuestros juegos por LAN.' : 'El proceso y su interfaz de control responden. Esto no garantiza una LAN funcional ni amigos conectados.';
    if (status === 'error') description = engine.message || 'El motor ha comunicado un error. Revisa el estado antes de volver a intentarlo.';
    text('room-status-description', description);
    text('room-destination', room.rendezvous || 'Sin destino configurado');
    text('room-public-key', room.rendezvousKey || '');
    $('room-key-detail').hidden = !room.rendezvousKey;
    const ip = actualIP(room);
    text('room-ip', ip || (inspection ? 'No se crea en inspección' : 'No disponible'));
    $('copy-ip').hidden = !ip;
    text('connect-label', inspection ? 'Revisar inspección' : 'Revisar conexión');
    $('connect-room').hidden = own;
    $('disconnect-room').hidden = !own;
    text('connect-hint', own ? (inspection ? 'Puedes detener el motor cuando quieras. En este modo no hay red virtual.' : 'Comprueba la IP y la conexión desde el juego. Detén el motor al terminar.') : connectionBlocker(room) || (inspection ? 'El siguiente paso inicia una inspección local, no una VPN. Revisa el destino antes de confirmar.' : 'Antes de iniciar el motor verás el destino, la clave pública y los riesgos de conexión.'));
    renderPeers(room);
  }

  function roomFingerprint(room) {
    return room ? JSON.stringify([room.id, room.name, room.rendezvous || '', room.rendezvousKey || '', state.engine.mode]) : '';
  }

  function syncControls() {
    const blocked = pending || !online || !state;
    document.querySelectorAll('[data-needs-state]').forEach((button) => { button.disabled = blocked; });
    for (const { button } of roomNodes.values()) button.disabled = pending;
    const room = currentRoom();
    $('connect-room').disabled = blocked || Boolean(connectionBlocker(room));
    $('disconnect-room').disabled = blocked || !roomOwnsEngine(room);
    $('share-room').disabled = blocked || !room;
    $('delete-room').disabled = blocked || !room || roomOwnsEngine(room);
    $('delete-room').title = roomOwnsEngine(room) ? 'Detén el motor antes de eliminar esta sala' : 'Eliminar la sala de este equipo';
    $('copy-ip').disabled = blocked || !actualIP(room) || copying;
    $('stop-engine').disabled = blocked || !activeEngine();
    $('retry-state').disabled = pending;
    for (const dialog of dialogs) {
      dialog.setAttribute('aria-busy', String(pending && dialog.open));
      dialog.querySelectorAll('button, input, textarea').forEach((node) => { node.disabled = pending; });
      dialog.querySelectorAll('button[type="submit"]').forEach((node) => { node.disabled = blocked; });
    }
    const confirmedRoom = connectSnapshot ? state?.rooms.find((item) => item.id === connectSnapshot.roomId) : null;
    const matches = confirmedRoom && roomFingerprint(confirmedRoom) === connectSnapshot.fingerprint;
    $('confirm-connect').disabled = blocked || !$('connect-ack').checked || !matches || Boolean(connectionBlocker(confirmedRoom));
    if ($('connect-dialog').open && connectSnapshot && !matches) {
      $('connect-ack').checked = false;
      showError($('connect-dialog'), 'La sala o el modo han cambiado. Cierra este paso y revisa la conexión de nuevo.');
    }
    if ($('delete-dialog').open) {
      const target = state?.rooms.find((item) => item.id === deleteRoomId);
      $('delete-form').querySelector('[type="submit"]').disabled = blocked || !target || roomOwnsEngine(target);
      if (!target) showError($('delete-dialog'), 'Esta sala ya no está guardada en este equipo.');
      else if (roomOwnsEngine(target)) showError($('delete-dialog'), 'Detén el motor antes de eliminar esta sala.');
    }
    $('copy-invite').disabled = pending || copying || !$('share-invite').value;
  }

  function render() {
    if (!state) { syncControls(); return; }
    renderRooms();
    renderEngine();
    const room = currentRoom();
    $('home-view').hidden = Boolean(room);
    $('room-view').hidden = !room;
    document.title = room ? `${room.name} — el ciber` : 'el ciber — Vuestras salas';
    if (room) renderRoom(room);
    syncControls();
  }

  function openDialog(id) {
    if (pending || !online || !state || dialogs.some((dialog) => dialog.open)) return false;
    const dialog = $(id);
    dialogOpeners.set(dialog, document.activeElement);
    showError(dialog);
    dialog.showModal();
    return true;
  }

  function setPending(value) {
    pending = value;
    syncControls();
  }

  async function mutation({ dialog, path, method = 'POST', body = {}, accepted, verify, message }) {
    if (pending || !online) return;
    if (dialog) showError(dialog);
    setPending(true);
    try {
      const result = await api(path, method, body);
      // Finish any older poll and read the committed state before selecting a
      // newly-created/imported room. An older snapshot must not clear it.
      const refreshed = await refreshState(true);
      if (accepted) accepted(result);
      render();
      if (refreshed && (!verify || verify(state, result))) notify(message);
      else notify(refreshed ? 'La solicitud se ha recibido. Revisa el estado del motor antes de continuar.' : 'La solicitud se ha recibido, pero no se ha podido verificar el estado actualizado.', true);
    } catch (error) {
      if (dialog?.open) showError(dialog, error.message);
      else notify(error.message, true);
      await refreshState(true);
    } finally {
      setPending(false);
      if (dialog && !dialog.open) restoreFocus(dialog);
    }
  }

  function restoreFocus(dialog) {
    const opener = dialogOpeners.get(dialog);
    if (opener?.isConnected && !opener.disabled && opener.getClientRects().length) opener.focus({ preventScroll: true });
    else $('workspace').focus({ preventScroll: true });
  }

  document.querySelectorAll('[data-open]').forEach((button) => {
    button.addEventListener('click', () => openDialog(button.dataset.open));
  });
  dialogs.forEach((dialog) => {
    dialog.querySelectorAll('[data-close]').forEach((button) => {
      button.addEventListener('click', () => { if (!pending) dialog.close(); });
    });
    dialog.addEventListener('cancel', (event) => { if (pending) event.preventDefault(); });
    dialog.addEventListener('close', () => {
      showError(dialog);
      if (dialog.id === 'share-dialog') { $('share-invite').value = ''; text('share-feedback', ''); }
      if (dialog.id === 'import-dialog') $('import-invite').value = '';
      if (dialog.id === 'connect-dialog') { $('connect-ack').checked = false; connectSnapshot = null; }
      if (dialog.id === 'delete-dialog') deleteRoomId = null;
      if (!pending) restoreFocus(dialog);
    });
  });

  $('brand-home').addEventListener('click', (event) => {
    event.preventDefault();
    if (!pending) selectRoom(null, true);
  });
  $('retry-state').addEventListener('click', () => refreshState());
  $('quit-app').addEventListener('click', async () => {
    if (pending || !online || !window.confirm('¿Cerrar El Ciber? Se detendrá la conexión de este equipo. Las salas se conservan.')) return;
    setPending(true);
    try {
      await api('/quit', 'POST', {});
      clearTimeout(pollTimer);
      online = false;
      $('connection-error').hidden = false;
      text('connection-error-text', 'El Ciber se ha cerrado. Puedes cerrar esta pestaña; tus salas están guardadas.');
      $('retry-state').hidden = true;
    } catch (error) { notify(error.message, true); }
    finally { setPending(false); }
  });

  $('create-form').addEventListener('submit', (event) => {
    event.preventDefault();
    const name = $('create-name').value.trim();
    if (!name) { showError($('create-dialog'), 'Escribe un nombre para la sala.'); $('create-name').focus(); return; }
    const body = { name, game: $('create-game').value.trim(), rendezvous: $('create-rendezvous').value.trim(), rendezvousKey: $('create-rendezvous-key').value.trim() };
    mutation({
      dialog: $('create-dialog'), path: '/rooms', body,
      accepted: (room) => {
        persistSelection(room.id);
        $('create-form').reset();
        $('create-form').querySelector('details').open = false;
        $('create-dialog').close();
      },
      verify: (next, room) => next.rooms.some((item) => item.id === room.id),
      message: 'Sala creada y guardada. No se ha iniciado ninguna conexión.',
    });
  });

  $('import-form').addEventListener('submit', (event) => {
    event.preventDefault();
    const invite = $('import-invite').value.trim();
    if (!invite) { showError($('import-dialog'), 'Pega la invitación completa.'); return; }
    mutation({
      dialog: $('import-dialog'), path: '/rooms/import', body: { invite },
      accepted: (room) => { persistSelection(room.id); $('import-invite').value = ''; $('import-dialog').close(); },
      verify: (next, room) => next.rooms.some((item) => item.id === room.id),
      message: 'Invitación guardada. No se ha iniciado ninguna conexión.',
    });
  });

  $('share-room').addEventListener('click', async () => {
    const room = currentRoom();
    if (!room || !openDialog('share-dialog')) return;
    const dialog = $('share-dialog');
    $('share-invite').value = '';
    $('share-invite').closest('.field').hidden = true;
    text('share-room-name', room.name);
    text('share-feedback', 'Preparando la invitación…');
    setPending(true);
    try {
      // The secret is fetched only in response to this explicit click. Never during polling.
      const result = await api(`/rooms/${encodeURIComponent(room.id)}/invite`, 'POST', {});
      if (typeof result.invite !== 'string' || !result.invite) throw new Error('No se ha recibido una invitación válida.');
      if (document.hidden) { dialog.close(); return; }
      $('share-invite').value = result.invite;
      $('share-invite').closest('.field').hidden = false;
      text('share-feedback', 'Al cerrar este diálogo se oculta la invitación. No se copia automáticamente.');
    } catch (error) {
      text('share-feedback', '');
      showError(dialog, error.message);
    } finally {
      setPending(false);
      if (dialog.open && $('share-invite').value) $('copy-invite').focus();
      else if (!dialog.open) restoreFocus(dialog);
    }
  });

  async function copyText(value, field = null) {
    if (!value || copying) return false;
    copying = true;
    syncControls();
    try {
      if (!navigator.clipboard?.writeText) throw new Error('Clipboard not available');
      await navigator.clipboard.writeText(value);
      return true;
    } catch {
      if (field) {
        field.focus();
        field.select();
        text('share-feedback', 'El navegador no ha permitido copiar. La invitación está seleccionada: usa Ctrl+C, ⌘C o la opción Copiar del dispositivo.');
      } else notify('El navegador no ha permitido copiar. Puedes seleccionar la IP que aparece en los datos de la sala.', true);
      return false;
    } finally {
      copying = false;
      syncControls();
    }
  }

  $('copy-invite').addEventListener('click', async () => {
    if (await copyText($('share-invite').value, $('share-invite'))) text('share-feedback', 'Invitación copiada. Compártela en privado; ocultarla aquí no borra el portapapeles.');
  });
  $('copy-ip').addEventListener('click', async () => {
    if (!online || pending) return;
    if (await copyText(actualIP(currentRoom()))) notify('IP virtual comunicada por EasyTier copiada.');
  });

  $('connect-room').addEventListener('click', () => {
    const room = currentRoom();
    if (!room || connectionBlocker(room) || !openDialog('connect-dialog')) return;
    const inspection = state.engine.mode === 'inspection';
    connectSnapshot = { roomId: room.id, fingerprint: roomFingerprint(room) };
    $('connect-ack').checked = false;
    text('confirm-room-name', room.name);
    text('confirm-destination', room.rendezvous);
    text('confirm-public-key', room.rendezvousKey || '');
    $('confirm-key-detail').hidden = !room.rendezvousKey;
    text('confirm-mode', inspection ? 'Inspección · No crea red virtual' : 'VPN · Puede crear adaptador y rutas');
    text('connect-description', inspection ? 'Vas a iniciar EasyTier en modo inspección, sin adaptador virtual ni rutas. Esto no conecta vuestros juegos por LAN.' : 'Vas a iniciar EasyTier para esta sala. El modo VPN puede crear un adaptador virtual y cambiar las rutas de red de este equipo.');
    text('confirm-risk', inspection ? 'El motor puede comunicarse con el destino loopback mostrado. Confía solo en nodos que conozcas y no compartas la invitación públicamente. No se creará una red virtual.' : 'Las personas con la invitación pueden acceder a servicios que expongas en la red virtual. Verifica el destino y su clave pública. En esta alpha, otro miembro con el secreto puede ser aceptado aunque su clave sea distinta. No desactives el cortafuegos de forma general.');
    text('confirm-connect-label', inspection ? 'Iniciar inspección' : 'Confirmar conexión');
    syncControls();
  });
  $('connect-ack').addEventListener('change', syncControls);
  $('connect-form').addEventListener('submit', (event) => {
    event.preventDefault();
    const room = state?.rooms.find((item) => item.id === connectSnapshot?.roomId);
    if (!room || !connectSnapshot || roomFingerprint(room) !== connectSnapshot.fingerprint) {
      showError($('connect-dialog'), 'La sala ha cambiado. Cierra este paso y revisa el destino de nuevo.');
      return;
    }
    const blocker = connectionBlocker(room);
    if (blocker) { showError($('connect-dialog'), blocker); return; }
    if (!$('connect-ack').checked) return;
    mutation({
      dialog: $('connect-dialog'), path: '/connect', body: { roomId: room.id, acknowledge: true },
      accepted: () => $('connect-dialog').close(),
      verify: (next) => next.engine.roomId === room.id && ['starting', 'running'].includes(next.engine.status),
      message: state.engine.mode === 'inspection' ? 'Inspección iniciada. No crea red virtual.' : 'Inicio del motor solicitado. Revisa su estado y comprueba la conexión desde el juego.',
    });
  });

  function disconnect() {
    if (!activeEngine()) return;
    mutation({ path: '/disconnect', verify: (next) => next.engine.status === 'stopped', message: 'Motor detenido. Tus salas siguen guardadas.' });
  }
  $('disconnect-room').addEventListener('click', disconnect);
  $('stop-engine').addEventListener('click', disconnect);

  $('delete-room').addEventListener('click', () => {
    const room = currentRoom();
    if (!room || roomOwnsEngine(room) || !openDialog('delete-dialog')) return;
    deleteRoomId = room.id;
    text('delete-room-name', room.name);
    syncControls();
  });
  $('delete-form').addEventListener('submit', (event) => {
    event.preventDefault();
    const room = state?.rooms.find((item) => item.id === deleteRoomId);
    if (!room || roomOwnsEngine(room)) return;
    mutation({
      dialog: $('delete-dialog'), path: `/rooms/${encodeURIComponent(room.id)}`, method: 'DELETE',
      accepted: () => { if (selectedRoomId === room.id) persistSelection(null); $('delete-dialog').close(); },
      verify: (next) => !next.rooms.some((item) => item.id === room.id),
      message: 'Sala eliminada de este equipo. Las invitaciones compartidas no se han revocado.',
    });
  });

  // Prevent secrets being restored from the back/forward cache or left in a hidden share dialog.
  document.addEventListener('visibilitychange', () => {
    if (document.hidden && $('share-dialog').open && !pending) $('share-dialog').close();
  });
  window.addEventListener('pagehide', () => {
    clearTimeout(pollTimer);
    $('share-invite').value = '';
    $('import-invite').value = '';
  });
  window.addEventListener('pageshow', (event) => { if (event.persisted) poll(); });

  // Decorative SVGs never add duplicate names to accessible controls.
  document.querySelectorAll('svg.icon').forEach((svg) => svg.setAttribute('aria-hidden', 'true'));
  syncControls();
  poll();
})();

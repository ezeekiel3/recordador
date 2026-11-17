// Audio Context and Synthesizer
let audioCtx = null;
let alarmLoopInterval = null;
let currentAlarmingItem = null;

function getAudioContext() {
  if (!audioCtx) {
    const AudioContextClass = window.AudioContext || window.webkitAudioContext;
    audioCtx = new AudioContextClass();
  }
  if (audioCtx.state === 'suspended') {
    audioCtx.resume();
  }
  return audioCtx;
}

// Synthesizer sounds using Web Audio API
function playSound(type) {
  try {
    const ctx = getAudioContext();
    const now = ctx.currentTime;

    switch (type) {
      case 'digital':
        // Digital alarm: 3 pairs of beeps
        for (let i = 0; i < 3; i++) {
          const t1 = now + i * 0.35;
          const osc1 = ctx.createOscillator();
          const gain1 = ctx.createGain();
          osc1.type = 'square';
          osc1.frequency.setValueAtTime(1100, t1);
          gain1.gain.setValueAtTime(0.12, t1);
          gain1.gain.exponentialRampToValueAtTime(0.001, t1 + 0.1);
          osc1.connect(gain1);
          gain1.connect(ctx.destination);
          osc1.start(t1);
          osc1.stop(t1 + 0.1);

          const t2 = t1 + 0.12;
          const osc2 = ctx.createOscillator();
          const gain2 = ctx.createGain();
          osc2.type = 'square';
          osc2.frequency.setValueAtTime(1400, t2);
          gain2.gain.setValueAtTime(0.12, t2);
          gain2.gain.exponentialRampToValueAtTime(0.001, t2 + 0.1);
          osc2.connect(gain2);
          gain2.connect(ctx.destination);
          osc2.start(t2);
          osc2.stop(t2 + 0.1);
        }
        break;

      case 'melodia':
        // Ascending harmonic notes (C5, E5, G5, C6)
        const notes = [523.25, 659.25, 783.99, 1046.5];
        notes.forEach((freq, idx) => {
          const t = now + idx * 0.14;
          const osc = ctx.createOscillator();
          const gain = ctx.createGain();
          osc.type = 'triangle';
          osc.frequency.setValueAtTime(freq, t);
          gain.gain.setValueAtTime(0.2, t);
          gain.gain.exponentialRampToValueAtTime(0.001, t + 0.3);
          osc.connect(gain);
          gain.connect(ctx.destination);
          osc.start(t);
          osc.stop(t + 0.35);
        });
        break;

      case 'ding':
        // Crisp high bell
        const oscD = ctx.createOscillator();
        const gainD = ctx.createGain();
        oscD.type = 'sine';
        oscD.frequency.setValueAtTime(1318.5, now); // E6
        gainD.gain.setValueAtTime(0.25, now);
        gainD.gain.exponentialRampToValueAtTime(0.001, now + 0.6);
        oscD.connect(gainD);
        gainD.connect(ctx.destination);
        oscD.start(now);
        oscD.stop(now + 0.6);
        break;

      case 'campana':
      default:
        // Pleasant bell chime
        const fundamental = 880; // A5
        [fundamental, fundamental * 1.5, fundamental * 2].forEach((freq, i) => {
          const osc = ctx.createOscillator();
          const gain = ctx.createGain();
          osc.type = 'sine';
          osc.frequency.setValueAtTime(freq, now);
          const vol = 0.2 / (i + 1);
          gain.gain.setValueAtTime(vol, now);
          gain.gain.exponentialRampToValueAtTime(0.0001, now + 1.2);
          osc.connect(gain);
          gain.connect(ctx.destination);
          osc.start(now);
          osc.stop(now + 1.3);
        });
        break;
    }
  } catch (err) {
    console.warn('AudioContext error:', err);
  }
}

function startAlarmLoop(type) {
  stopAlarmLoop();
  playSound(type);
  alarmLoopInterval = setInterval(() => {
    playSound(type);
  }, 2200);
}

function stopAlarmLoop() {
  if (alarmLoopInterval) {
    clearInterval(alarmLoopInterval);
    alarmLoopInterval = null;
  }
}

// State
let allReminders = [];
let currentFilter = 'all';
let currentSearch = '';

// Date Utilities
function formatDateForInput(date) {
  const pad = (n) => String(n).padStart(2, '0');
  const yyyy = date.getFullYear();
  const mm = pad(date.getMonth() + 1);
  const dd = pad(date.getDate());
  const hh = pad(date.getHours());
  const min = pad(date.getMinutes());
  return `${yyyy}-${mm}-${dd}T${hh}:${min}`;
}

function setDefaultDatetime() {
  const now = new Date();
  now.setMinutes(now.getMinutes() + 15);
  document.getElementById('input-datetime').value = formatDateForInput(now);
}

// Preset Handlers
function setupPresets() {
  document.querySelectorAll('.btn-preset').forEach((btn) => {
    btn.addEventListener('click', () => {
      const mins = parseInt(btn.getAttribute('data-min'), 10);
      const target = new Date();
      target.setMinutes(target.getMinutes() + mins);
      document.getElementById('input-datetime').value = formatDateForInput(target);
      showToast(`Alarma fijada para dentro de ${mins} minutos`, 'info');
    });
  });

  document.getElementById('btn-preset-today-evening').addEventListener('click', () => {
    const target = new Date();
    target.setHours(18, 0, 0, 0);
    if (target.getTime() <= Date.now()) {
      target.setDate(target.getDate() + 1);
    }
    document.getElementById('input-datetime').value = formatDateForInput(target);
    showToast(`Alarma fijada para las 18:00`, 'info');
  });

  document.getElementById('btn-preset-tomorrow-morning').addEventListener('click', () => {
    const target = new Date();
    target.setDate(target.getDate() + 1);
    target.setHours(9, 0, 0, 0);
    document.getElementById('input-datetime').value = formatDateForInput(target);
    showToast(`Alarma fijada para mañana a las 09:00`, 'info');
  });
}

// API Calls
async function fetchReminders() {
  try {
    const res = await fetch('/api/reminders');
    if (!res.ok) throw new Error('Error al obtener recordatorios');
    allReminders = await res.json();
    renderReminders();
    updateCounts();
  } catch (err) {
    console.error('Error fetching reminders:', err);
  }
}

async function createReminder(data) {
  try {
    const res = await fetch('/api/reminders', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error('Error al guardar recordatorio');
    const created = await res.json();
    showToast('¡Recordatorio guardado con éxito!', 'success');
    document.getElementById('reminder-form').reset();
    setDefaultDatetime();
    await fetchReminders();
  } catch (err) {
    showToast('Error al guardar: ' + err.message, 'error');
  }
}

async function updateReminder(id, data) {
  try {
    const res = await fetch(`/api/reminders/${id}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
    if (!res.ok) throw new Error('Error al actualizar recordatorio');
    showToast('Recordatorio actualizado correctamente', 'success');
    closeEditModal();
    await fetchReminders();
  } catch (err) {
    showToast('Error al actualizar: ' + err.message, 'error');
  }
}

async function deleteReminder(id) {
  if (!confirm('¿Estás seguro de que deseas eliminar este recordatorio?')) {
    return;
  }
  try {
    const res = await fetch(`/api/reminders/${id}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Error al eliminar');
    showToast('Recordatorio eliminado', 'info');
    await fetchReminders();
  } catch (err) {
    showToast('Error al eliminar: ' + err.message, 'error');
  }
}

async function toggleComplete(id) {
  try {
    const res = await fetch(`/api/reminders/${id}/complete`, { method: 'POST' });
    if (!res.ok) throw new Error('Error al cambiar estado');
    await fetchReminders();
  } catch (err) {
    showToast('Error: ' + err.message, 'error');
  }
}

async function snoozeReminder(id, minutes = 5) {
  try {
    const res = await fetch(`/api/reminders/${id}/snooze`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ minutes }),
    });
    if (!res.ok) throw new Error('Error al posponer');
    showToast(`Pospuesto por ${minutes} minutos`, 'info');
    closeAlarmModal();
    await fetchReminders();
  } catch (err) {
    showToast('Error al posponer: ' + err.message, 'error');
  }
}

async function dismissAlarm(id) {
  try {
    await fetch(`/api/alarms/${id}/dismiss`, { method: 'POST' });
    closeAlarmModal();
    await fetchReminders();
  } catch (err) {
    console.error('Error dismissing alarm:', err);
  }
}

// Render Reminders
function renderReminders() {
  const container = document.getElementById('reminders-list');
  const now = new Date();

  let filtered = allReminders.filter((r) => {
    // Tab filter
    if (currentFilter === 'pending' && r.completed) return false;
    if (currentFilter === 'completed' && !r.completed) return false;

    // Search filter
    if (currentSearch.trim() !== '') {
      const q = currentSearch.toLowerCase();
      const matchTitle = (r.title || '').toLowerCase().includes(q);
      const matchText = (r.text || '').toLowerCase().includes(q);
      if (!matchTitle && !matchText) return false;
    }
    return true;
  });

  if (filtered.length === 0) {
    container.innerHTML = `
      <div class="empty-state">
        <div class="empty-icon">📝</div>
        <h3>No hay recordatorios que mostrar</h3>
        <p>${
          currentSearch ? 'No se encontraron resultados para tu búsqueda.' : 'Crea uno nuevo usando el formulario.'
        }</p>
      </div>
    `;
    return;
  }

  container.innerHTML = filtered
    .map((r) => {
      const dueDate = new Date(r.due_time);
      const isPast = dueDate.getTime() < now.getTime();
      let statusClass = 'pending';
      let statusLabel = 'Pendiente';

      if (r.completed) {
        statusClass = 'completed';
        statusLabel = 'Completado';
      } else if (isPast) {
        statusClass = 'overdue';
        statusLabel = '¡Vencido!';
      }

      // Friendly countdown / formatted time
      const timeDiff = dueDate.getTime() - now.getTime();
      let countdownText = '';
      if (r.completed) {
        countdownText = 'Completado';
      } else if (timeDiff > 0) {
        const mins = Math.floor(timeDiff / (1000 * 60));
        const hours = Math.floor(mins / 60);
        const days = Math.floor(hours / 24);
        if (days > 0) countdownText = `En ${days}d ${hours % 24}h`;
        else if (hours > 0) countdownText = `En ${hours}h ${mins % 60}m`;
        else countdownText = `En ${mins} min`;
      } else {
        const minsPast = Math.floor(-timeDiff / (1000 * 60));
        countdownText = minsPast === 0 ? 'Ahora' : `Hace ${minsPast} min`;
      }

      const formattedDate = dueDate.toLocaleString('es-ES', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });

      const soundIcons = {
        campana: '🔔 Campana',
        digital: '⏱️ Despertador',
        melodia: '🎵 Melodía',
        ding: '✨ Ding',
      };
      const soundName = soundIcons[r.sound_type] || '🔔 Alarma';

      return `
        <div class="reminder-item ${statusClass}" data-id="${r.id}">
          <div class="item-top">
            <div class="item-main">
              <div class="item-title-row">
                <span class="item-title">${escapeHtml(r.title)}</span>
              </div>
              ${r.text ? `<p class="item-text">${escapeHtml(r.text)}</p>` : ''}
              
              <div class="item-badge-row">
                <span class="badge badge-time">📅 ${formattedDate}</span>
                <span class="badge ${r.completed ? 'badge-completed' : isPast ? 'badge-overdue' : 'badge-countdown'}">
                  ${countdownText}
                </span>
                <span class="badge badge-sound">${soundName}</span>
              </div>
            </div>
          </div>

          <div class="item-actions">
            <button class="btn-icon-action btn-action-complete" onclick="toggleComplete('${r.id}')" title="${
        r.completed ? 'Marcar como pendiente' : 'Marcar como completado'
      }">
              ${r.completed ? '↩️ Reactivar' : '✅ Completar'}
            </button>
            <button class="btn-icon-action btn-action-edit" onclick="openEditModal('${r.id}')" title="Editar">
              ✏️ Editar
            </button>
            <button class="btn-icon-action" onclick="playSound('${r.sound_type}')" title="Probar sonido">
              🔊 Probar
            </button>
            <button class="btn-icon-action btn-action-delete" onclick="deleteReminder('${r.id}')" title="Eliminar">
              🗑️ Eliminar
            </button>
          </div>
        </div>
      `;
    })
    .join('');
}

function updateCounts() {
  const total = allReminders.length;
  const completed = allReminders.filter((r) => r.completed).length;
  const pending = total - completed;

  document.getElementById('count-all').textContent = total;
  document.getElementById('count-pending').textContent = pending;
  document.getElementById('count-completed').textContent = completed;
}

// Edit Modal
window.openEditModal = function (id) {
  const item = allReminders.find((r) => r.id === id);
  if (!item) return;

  document.getElementById('edit-id').value = item.id;
  document.getElementById('edit-title').value = item.title;
  document.getElementById('edit-text').value = item.text || '';
  document.getElementById('edit-sound').value = item.sound_type || 'campana';

  const dueDate = new Date(item.due_time);
  document.getElementById('edit-datetime').value = formatDateForInput(dueDate);

  document.getElementById('modal-edit').classList.remove('hidden');
};

function closeEditModal() {
  document.getElementById('modal-edit').classList.add('hidden');
}

// Alarm Modal
function showAlarmModal(item) {
  currentAlarmingItem = item;
  document.getElementById('alarm-title').textContent = item.title;
  document.getElementById('alarm-text').textContent = item.text || '(Sin descripción adicional)';

  const d = new Date(item.due_time);
  document.getElementById('alarm-time').textContent = d.toLocaleTimeString('es-ES', {
    hour: '2-digit',
    minute: '2-digit',
  });

  document.getElementById('modal-alarm').classList.remove('hidden');
  startAlarmLoop(item.sound_type || 'campana');
}

function closeAlarmModal() {
  stopAlarmLoop();
  currentAlarmingItem = null;
  document.getElementById('modal-alarm').classList.add('hidden');
}

// Alarm Polling (checks server every 1.5 seconds)
async function pollAlarms() {
  try {
    const res = await fetch('/api/alarms/active');
    if (!res.ok) return;
    const active = await res.json();

    if (active && active.length > 0) {
      const topAlarm = active[0];
      // If modal is not already showing this item
      if (!currentAlarmingItem || currentAlarmingItem.id !== topAlarm.id) {
        showAlarmModal(topAlarm);
      }
    }
  } catch (err) {
    // silent catch for background polling
  }
}

// Toast Helper
function showToast(message, type = 'info') {
  const container = document.getElementById('toast-container');
  const toast = document.createElement('div');
  toast.className = `toast toast-${type}`;
  toast.textContent = message;
  container.appendChild(toast);

  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transition = 'opacity 0.3s ease';
    setTimeout(() => toast.remove(), 300);
  }, 3500);
}

function escapeHtml(str) {
  if (!str) return '';
  return str
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');
}

// Event Listeners Initialization
document.addEventListener('DOMContentLoaded', () => {
  setDefaultDatetime();
  setupPresets();
  fetchReminders();

  // Create form submission
  document.getElementById('reminder-form').addEventListener('submit', (e) => {
    e.preventDefault();
    const title = document.getElementById('input-title').value.trim();
    const text = document.getElementById('input-text').value.trim();
    const dtVal = document.getElementById('input-datetime').value;
    const soundType = document.getElementById('select-sound').value;

    if (!title || !dtVal) {
      showToast('Por favor completa el título y la fecha/hora', 'error');
      return;
    }

    const dueTime = new Date(dtVal).toISOString();
    createReminder({
      title,
      text,
      due_time: dueTime,
      sound_type: soundType,
    });
  });

  // Clear form
  document.getElementById('btn-clear-form').addEventListener('click', () => {
    document.getElementById('reminder-form').reset();
    setDefaultDatetime();
  });

  // Sound preview buttons
  document.getElementById('btn-preview-sound').addEventListener('click', () => {
    const sound = document.getElementById('select-sound').value;
    playSound(sound);
    // Also test system sound via backend
    fetch('/api/test-sound', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ sound_type: sound }),
    });
  });

  document.getElementById('btn-preview-edit-sound').addEventListener('click', () => {
    const sound = document.getElementById('edit-sound').value;
    playSound(sound);
  });

  document.getElementById('btn-test-sound-header').addEventListener('click', () => {
    playSound('campana');
    fetch('/api/test-sound', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ sound_type: 'campana' }),
    });
    showToast('Reproduciendo sonido de prueba...', 'info');
  });

  const btnExit = document.getElementById('btn-exit-app');
  if (btnExit) {
    btnExit.addEventListener('click', async () => {
      if (confirm('¿Deseas cerrar la aplicación completamente?')) {
        try {
          await fetch('/api/exit');
        } catch (_) {}
        window.close();
      }
    });
  }

  // Edit form submit
  document.getElementById('form-edit').addEventListener('submit', (e) => {
    e.preventDefault();
    const id = document.getElementById('edit-id').value;
    const title = document.getElementById('edit-title').value.trim();
    const text = document.getElementById('edit-text').value.trim();
    const dtVal = document.getElementById('edit-datetime').value;
    const soundType = document.getElementById('edit-sound').value;

    const dueTime = new Date(dtVal).toISOString();
    updateReminder(id, {
      title,
      text,
      due_time: dueTime,
      sound_type: soundType,
    });
  });

  document.getElementById('btn-close-edit').addEventListener('click', closeEditModal);
  document.getElementById('btn-cancel-edit').addEventListener('click', closeEditModal);

  // Alarm modal actions
  document.getElementById('btn-alarm-dismiss').addEventListener('click', () => {
    if (currentAlarmingItem) {
      dismissAlarm(currentAlarmingItem.id);
    }
  });

  document.getElementById('btn-alarm-snooze').addEventListener('click', () => {
    if (currentAlarmingItem) {
      snoozeReminder(currentAlarmingItem.id, 5);
    }
  });

  document.getElementById('btn-alarm-complete').addEventListener('click', async () => {
    if (currentAlarmingItem) {
      const id = currentAlarmingItem.id;
      await toggleComplete(id);
      closeAlarmModal();
    }
  });

  // Filter tabs
  document.querySelectorAll('.filter-btn').forEach((btn) => {
    btn.addEventListener('click', () => {
      document.querySelectorAll('.filter-btn').forEach((b) => b.classList.remove('active'));
      btn.classList.add('active');
      currentFilter = btn.getAttribute('data-filter');
      renderReminders();
    });
  });

  // Search input
  document.getElementById('search-input').addEventListener('input', (e) => {
    currentSearch = e.target.value;
    renderReminders();
  });

  // Re-render countdowns every 15 seconds
  setInterval(() => {
    renderReminders();
  }, 15000);

  // Poll for active alarms every 1.5 seconds
  setInterval(pollAlarms, 1500);

  // Unlock AudioContext on first user interaction anywhere
  const unlockAudio = () => {
    getAudioContext();
    document.removeEventListener('click', unlockAudio);
    document.removeEventListener('keydown', unlockAudio);
  };
  document.addEventListener('click', unlockAudio);
  document.addEventListener('keydown', unlockAudio);
});

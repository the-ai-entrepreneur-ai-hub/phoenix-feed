const API_URL = "http://localhost:8080/v1/incidents/active";

const map = L.map("map", { zoomControl: true }).setView([33.4484, -112.074], 11);
L.tileLayer("https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png", {
  maxZoom: 19,
  attribution: "&copy; OpenStreetMap contributors"
}).addTo(map);

const markers = L.layerGroup().addTo(map);
const list = document.querySelector("#incidents");
const statusEl = document.querySelector("#status");
const metaEl = document.querySelector("#meta");
const refreshButton = document.querySelector("#refresh");

refreshButton.addEventListener("click", () => {
  loadIncidents();
});

loadIncidents();
setInterval(loadIncidents, 60000);

async function loadIncidents() {
  statusEl.textContent = "Updating";
  try {
    const response = await fetch(API_URL, { headers: { Accept: "application/json" } });
    if (!response.ok) {
      throw new Error(`API returned ${response.status}`);
    }
    const payload = await response.json();
    renderMeta(payload.meta || {});
    renderIncidents(payload.incidents || []);
    statusEl.textContent = `Updated ${new Date().toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`;
  } catch (error) {
    statusEl.textContent = `Unable to reach localhost:8080: ${error.message}`;
  }
}

function renderMeta(meta) {
  const parts = [];
  if (meta.source_last_success_at) {
    parts.push(`source ${new Date(meta.source_last_success_at).toLocaleTimeString([], { hour: "numeric", minute: "2-digit" })}`);
  }
  if (typeof meta.data_age_seconds === "number") {
    parts.push(`${meta.data_age_seconds}s old`);
  }
  if (meta.parser_version) {
    parts.push(meta.parser_version);
  }
  metaEl.textContent = parts.length ? parts.join(" · ") : "No successful source poll yet";
}

function renderIncidents(incidents) {
  markers.clearLayers();
  list.textContent = "";

  if (incidents.length === 0) {
    const empty = document.createElement("div");
    empty.className = "empty";
    empty.textContent = "No active incidents in the current cache.";
    list.append(empty);
    return;
  }

  const bounds = [];
  for (const incident of incidents) {
    const item = renderIncidentRow(incident);
    list.append(item);

    if (Number.isFinite(incident.lat) && Number.isFinite(incident.lon)) {
      const marker = L.marker([incident.lat, incident.lon]).addTo(markers);
      marker.bindPopup(popupHTML(incident));
      bounds.push([incident.lat, incident.lon]);
      item.addEventListener("click", () => {
        map.setView([incident.lat, incident.lon], Math.max(map.getZoom(), 14));
        marker.openPopup();
      });
    }
  }

  if (bounds.length) {
    map.fitBounds(bounds, { padding: [28, 28], maxZoom: 13 });
  }
}

function renderIncidentRow(incident) {
  const item = document.createElement("article");
  item.className = "incident";
  const title = incident.nature_desc || incident.nature_code || "Incident";
  item.innerHTML = `
    <h2>${escapeHTML(title)}</h2>
    <dl>
      <dt>ID</dt><dd>${escapeHTML(incident.source || "")}/${escapeHTML(incident.incident_id || "")}</dd>
      <dt>Location</dt><dd>${escapeHTML(incident.location_text || "Unknown")}</dd>
      <dt>Channel</dt><dd>${escapeHTML(incident.channel || "Unknown")}</dd>
      <dt>Last seen</dt><dd>${formatTime(incident.last_seen_at)}</dd>
      <dt>Units</dt><dd>${escapeHTML(formatUnits(incident.units || []))}</dd>
    </dl>
  `;
  return item;
}

function popupHTML(incident) {
  const title = incident.nature_desc || incident.nature_code || "Incident";
  return `
    <strong>${escapeHTML(title)}</strong><br>
    ${escapeHTML(incident.location_text || "Unknown location")}<br>
    ${escapeHTML(formatUnits(incident.units || []))}
  `;
}

function formatUnits(units) {
  if (!units.length) {
    return "None listed";
  }
  return units.map((unit) => `${unit.Unit || unit.unit}: ${unit.Status || unit.status || ""}`.trim()).join(", ");
}

function formatTime(value) {
  if (!value) {
    return "Unknown";
  }
  return new Date(value).toLocaleString([], {
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit"
  });
}

function escapeHTML(value) {
  return String(value)
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#039;");
}

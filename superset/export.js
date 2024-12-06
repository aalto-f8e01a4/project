const { promises: fs } = require("fs");
const path = require("path");

const API_BASE = "http://superset.superset.svc.cluster.local:8088/api/v1";
const AUTH_TOKEN =
  "Bearer eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJmcmVzaCI6dHJ1ZSwiaWF0IjoxNzMzNDMxODI4LCJqdGkiOiJiYzk4MDYwNS1jNGNjLTQ2OWItODBkNi1jNjk5M2ZkZWIyMjEiLCJ0eXBlIjoiYWNjZXNzIiwic3ViIjoxLCJuYmYiOjE3MzM0MzE4MjgsImV4cCI6MTczMzQzMjcyOH0.dxbNbMtMRsovK8FnhP2RguyzFfuzke1OIzZ55qcEUmk";

async function exportData(endpoint, outputPath) {
  const res = await fetch(`${API_BASE}/${endpoint}`, {
    headers: { Authorization: AUTH_TOKEN },
  });

  if (!res.ok) {
    console.error(`Failed to fetch ${endpoint}: ${res.statusText}`);
    return;
  }

  const data = await res.json();
  await fs.writeFile(outputPath, JSON.stringify(data, null, 2));
  console.log(`Exported: ${outputPath}`);
}

async function exportDashboard(dashboardName, dashboardId) {
  const baseDir = path.join("exported", dashboardName);
  await fs.mkdir(baseDir, { recursive: true });

  await exportData(
    `dashboard/${dashboardId}`,
    path.join(baseDir, "dashboard.json")
  );

  const chartsDir = path.join(baseDir, "charts");
  await fs.mkdir(chartsDir, { recursive: true });
  const charts = await fetch(`${API_BASE}/dashboard/${dashboardId}/charts`, {
    headers: { Authorization: AUTH_TOKEN },
  }).then((res) => res.json());
  for (const chart of charts.result) {
    await exportData(
      `chart/${chart.id}`,
      path.join(chartsDir, `${chart.slice_name.replace(/\//g, "_")}.json`)
    );
  }

  const datasetsDir = path.join(baseDir, "datasets");
  await fs.mkdir(datasetsDir, { recursive: true });
  const datasets = await fetch(
    `${API_BASE}/dashboard/${dashboardId}/datasets`,
    { headers: { Authorization: AUTH_TOKEN } }
  ).then((res) => res.json());
  for (const dataset of datasets.result) {
    await fs.writeFile(
      path.join(datasetsDir, `${dataset.table_name.replace(/\//g, "_")}.json`),
      JSON.stringify(dataset, null, 2)
    );
  }
}

(async () => {
  const dashboards = await fetch(`${API_BASE}/dashboard/`, {
    headers: { Authorization: AUTH_TOKEN },
  }).then((res) => res.json());
  for (const dashboard of dashboards.result) {
    await exportDashboard(dashboard.dashboard_title, dashboard.id);
  }
})();

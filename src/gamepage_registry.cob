>>SOURCE FORMAT FREE
IDENTIFICATION DIVISION.
PROGRAM-ID. GAMEPAGE-REGISTRY-BUILDER.

ENVIRONMENT DIVISION.
INPUT-OUTPUT SECTION.
FILE-CONTROL.
    SELECT ROUTE-FILE ASSIGN TO "runtime/routes.tsv"
        ORGANIZATION IS LINE SEQUENTIAL.
    SELECT POLICY-FILE ASSIGN TO "runtime/policy.tsv"
        ORGANIZATION IS LINE SEQUENTIAL.
    SELECT ARCHITECTURE-FILE ASSIGN TO "runtime/architecture.json"
        ORGANIZATION IS LINE SEQUENTIAL.
    SELECT STATUS-CLIENT-FILE ASSIGN TO "assets/status.js"
        ORGANIZATION IS LINE SEQUENTIAL.

DATA DIVISION.
FILE SECTION.
FD ROUTE-FILE.
01 ROUTE-LINE PIC X(512).
FD POLICY-FILE.
01 POLICY-LINE PIC X(256).
FD ARCHITECTURE-FILE.
01 ARCHITECTURE-LINE PIC X(1024).
FD STATUS-CLIENT-FILE.
01 STATUS-CLIENT-LINE PIC X(4096).

PROCEDURE DIVISION.
MAIN.
    OPEN OUTPUT ROUTE-FILE POLICY-FILE ARCHITECTURE-FILE STATUS-CLIENT-FILE
    PERFORM WRITE-ROUTES
    PERFORM WRITE-POLICY
    PERFORM WRITE-ARCHITECTURE
    PERFORM WRITE-STATUS-CLIENT
    CLOSE ROUTE-FILE POLICY-FILE ARCHITECTURE-FILE STATUS-CLIENT-FILE
    DISPLAY "COBOL application registry and runtime policy generated."
    STOP RUN.

WRITE-ROUTES.
    WRITE ROUTE-LINE FROM
        "# key|name|prefix|upstream-env|default-upstream|health-path"
    WRITE ROUTE-LINE FROM
        "trump|Trump vs. Shakespeare|/play/trump|TRUMP_UPSTREAM|http://trump-vs-shakespeare:8000|/readyz"
    WRITE ROUTE-LINE FROM
        "golf|Crazy Mini Golf|/play/golf|GOLF_UPSTREAM|http://crazy-mini-golf:8080|/healthz"
    WRITE ROUTE-LINE FROM
        "race|Crazy Race|/play/race|RACE_UPSTREAM|http://crazy-race:8080|/health".

WRITE-POLICY.
    WRITE POLICY-LINE FROM "# COBOL-owned runtime policy"
    WRITE POLICY-LINE FROM "STATUS_CACHE_TTL|5s"
    WRITE POLICY-LINE FROM "MAX_GAMES|10".

WRITE-ARCHITECTURE.
    WRITE ARCHITECTURE-LINE FROM "{"
    WRITE ARCHITECTURE-LINE FROM
        '  "applicationOwner": "COBOL",'
    WRITE ARCHITECTURE-LINE FROM
        '  "decisionEngine": "GnuCOBOL",'
    WRITE ARCHITECTURE-LINE FROM
        '  "transportLayer": "Go",'
    WRITE ARCHITECTURE-LINE FROM
        '  "cobolResponsibilities": ['
    WRITE ARCHITECTURE-LINE FROM
        '    "route registry", "runtime policy", "configuration decisions",'
    WRITE ARCHITECTURE-LINE FROM
        '    "health aggregation", "maintenance mode", "readiness",'
    WRITE ARCHITECTURE-LINE FROM
        '    "launchability decisions", "status JSON", "launcher generation"'
    WRITE ARCHITECTURE-LINE FROM "  ],"
    WRITE ARCHITECTURE-LINE FROM
        '  "goResponsibilities": ["HTTP transport", "reverse proxy", "WebSockets", "service probes", "OS signals"]'
    WRITE ARCHITECTURE-LINE FROM "}".

WRITE-STATUS-CLIENT.
    WRITE STATUS-CLIENT-LINE FROM '"use strict";'
    WRITE STATUS-CLIENT-LINE FROM 'const statusNodes = new Map(Array.from(document.querySelectorAll("[data-status]")).map((node) => [node.dataset.status, node]));'
    WRITE STATUS-CLIENT-LINE FROM 'function setStatus(key, game, mode) {'
    WRITE STATUS-CLIENT-LINE FROM '  const node = statusNodes.get(key); if (!node) return;'
    WRITE STATUS-CLIENT-LINE FROM '  const launch = node.closest(".game-card")?.querySelector(".launch");'
    WRITE STATUS-CLIENT-LINE FROM '  const launchable = game?.launchable === true;'
    WRITE STATUS-CLIENT-LINE FROM '  node.classList.remove("checking", "up", "down"); node.classList.add(launchable ? "up" : "down");'
    WRITE STATUS-CLIENT-LINE FROM '  const label = node.querySelector("[data-status-text]");'
    WRITE STATUS-CLIENT-LINE FROM '  let text = "OFFLINE";'
    WRITE STATUS-CLIENT-LINE FROM '  if (mode === "maintenance") text = "MAINTENANCE";'
    WRITE STATUS-CLIENT-LINE FROM '  else if (game?.reason === "decision-engine-unavailable") text = "CORE ERROR";'
    WRITE STATUS-CLIENT-LINE FROM '  else if (launchable) text = `ONLINE${Number.isFinite(game.latencyMs) ? ` / ${game.latencyMs}MS` : ""}`;'
    WRITE STATUS-CLIENT-LINE FROM '  if (label) label.textContent = text;'
    WRITE STATUS-CLIENT-LINE FROM '  if (!launch) return; if (!launch.dataset.target) launch.dataset.target = launch.getAttribute("href") || "";'
    WRITE STATUS-CLIENT-LINE FROM '  if (launchable) { launch.setAttribute("href", launch.dataset.target); launch.removeAttribute("aria-disabled"); } else { launch.removeAttribute("href"); launch.setAttribute("aria-disabled", "true"); }'
    WRITE STATUS-CLIENT-LINE FROM '}'
    WRITE STATUS-CLIENT-LINE FROM 'async function refreshStatuses() {'
    WRITE STATUS-CLIENT-LINE FROM '  try {'
    WRITE STATUS-CLIENT-LINE FROM '    const response = await fetch("/api/status", {cache: "no-store", headers: {Accept: "application/json"}});'
    WRITE STATUS-CLIENT-LINE FROM '    if (!response.ok) throw new Error("status request failed"); const payload = await response.json();'
    WRITE STATUS-CLIENT-LINE FROM '    for (const [key, game] of Object.entries(payload.games || {})) setStatus(key, game, payload.mode);'
    WRITE STATUS-CLIENT-LINE FROM '  } catch (_) { for (const key of statusNodes.keys()) setStatus(key, {launchable:false, reason:"decision-engine-unavailable"}, "fail-safe"); }'
    WRITE STATUS-CLIENT-LINE FROM '}'
    WRITE STATUS-CLIENT-LINE FROM 'refreshStatuses(); setInterval(refreshStatuses, 30000);'.

// web/src/gi-bootstrap.ts
import("./chunks/app-j6242z2n.js").catch(() => {
  const host = document.getElementById("app");
  if (host)
    host.textContent = "Unable to load Gi. Reload the page to retry.";
});

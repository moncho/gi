// Retain Gi's measured resize safety without changing pinned/supplied bytes.
// Piclaw's installed renderer measures only on content/expansion changes, so a
// wrapped paragraph can lose its disclosure after a container-only resize.
export function patchPinnedStatusResize(source) {
 const edits=[
  ["import { html, useCallback, useEffect, useMemo, useRef, useState }", "import { usePreviewOverflow } from '../gi-preview-overflow.js';\nimport { html, useCallback, useEffect, useMemo, useRef, useState }"],
  ["const [expandedPanels, setExpandedPanels] = useState(new Set());", "const [expandedPanels, setExpandedPanels] = useState(new Set());\n    const previewOverflow = usePreviewOverflow(draft, thought, expandedPanels);"],
  ["const measuredOverflow = Boolean(panelKey && overflowingPanels[panelKey]);", "const measuredOverflow = panelKey === 'thought' || panelKey === 'draft' ? previewOverflow.overflow[panelKey] : Boolean(panelKey && overflowingPanels[panelKey]);"],
  ["if (!panelKey) return;\n                        if (node)", "if (!panelKey) return;\n                        previewOverflow.refs[panelKey]?.(node);\n                        if (node)"],
 ];
 for(const [from,to]of edits){if(source.split(from).length!==2)throw Error('Pinned status resize anchor changed: '+from);source=source.replace(from,to);}
 return source;
}

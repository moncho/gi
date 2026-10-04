import { html } from '../vendor/preact-htm.js';

export type DisclosureTriangleDirection = 'right' | 'down' | 'up' | 'left';

const TRIANGLE_POINTS: Record<DisclosureTriangleDirection, string> = {
  right: '3 2 8 5 3 8',
  down: '2 3 8 3 5 8',
  up: '2 7 8 7 5 2',
  left: '7 2 2 5 7 8',
};

function normalizeDirection(direction: unknown): DisclosureTriangleDirection {
  return direction === 'down' || direction === 'up' || direction === 'left' ? direction : 'right';
}

function classNames(direction: DisclosureTriangleDirection, extraClass = ''): string {
  return [
    'ui-disclosure-triangle',
    `ui-disclosure-triangle-${direction}`,
    extraClass.trim(),
  ].filter(Boolean).join(' ');
}

export function renderDisclosureTriangle(direction: DisclosureTriangleDirection = 'right', extraClass = '') {
  const normalized = normalizeDirection(direction);
  return html`
    <svg class=${classNames(normalized, extraClass)} viewBox="0 0 10 10" aria-hidden="true" focusable="false">
      <polygon points=${TRIANGLE_POINTS[normalized]}></polygon>
    </svg>
  `;
}

export function disclosureTriangleSvgString(direction: DisclosureTriangleDirection = 'right', extraClass = ''): string {
  const normalized = normalizeDirection(direction);
  return `<svg class="${classNames(normalized, extraClass)}" viewBox="0 0 10 10" aria-hidden="true" focusable="false"><polygon points="${TRIANGLE_POINTS[normalized]}"></polygon></svg>`;
}

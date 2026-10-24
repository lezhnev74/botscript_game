export interface Fit {
  cell: number;
  offsetX: number;
  offsetY: number;
}

/** Largest square cell size that fits a cols x rows map into the stage, with centring offsets. */
export function fitCells(stageW: number, stageH: number, cols: number, rows: number): Fit {
  if (cols <= 0 || rows <= 0) return { cell: 0, offsetX: 0, offsetY: 0 };
  const cell = Math.min(stageW / cols, stageH / rows);
  return { cell, offsetX: (stageW - cell * cols) / 2, offsetY: (stageH - cell * rows) / 2 };
}

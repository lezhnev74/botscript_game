/** Columns shown with an icon header on both the TV sidebar and the leaderboard. */
export const ICON_COLUMNS = ["kills", "deaths", "coins"] as const;
export type IconColumn = (typeof ICON_COLUMNS)[number];

export function isIconColumn(c: string): c is IconColumn {
  return (ICON_COLUMNS as readonly string[]).includes(c);
}

/** TV sidebar columns: title and fixed width (ch). */
export const TV_COLUMNS: readonly (readonly [string, number])[] = [["name", 13], ["HP", 8], ["kills", 5], ["deaths", 5], ["coins", 5], ["score", 7]];

/** Leaderboard column titles. */
export const LEADERBOARD_COLUMNS: readonly string[] = ["#", "name", "score", "kills", "deaths", "coins"];

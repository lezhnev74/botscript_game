// Tracker: hunter - chase the enemy you scan and guess which side it dodges to.
// num[0] = direction I'm tracking, num[1] = sidestep I tried, num[2] = 1 if I just sidestepped from an enemy at 2, 2 = retreated, sidestep next
function loop() {
  if (num[2] == 2) { num[2] = 0; move(pick(left(num[0]), right(num[0]))); return; }   // retreated last beat: sidestep now
  if (blocked(num[0])) num[0] = pick(left(num[0]), right(num[0]));   // wall ahead: turn instead of bumping
  scan(num[0]);
  if (bullet == 3) { dodge(num[0]); return; }
  if (enemy == 3 && loaded) { num[2] = 0; fire(num[0]); return; }
  if (enemy == 2) { num[2] = 1; num[1] = pick(left(num[0]), right(num[0])); move(num[1]); return; }
  if (num[2] == 1 && enemy == 1) {        // it was 2, I stepped, now 1: wrong side, cross over
    num[2] = 0; move(back(num[1])); move(back(num[1]));
    return;
  }
  num[2] = 0;
  if (enemy == 0) num[0] = pick(N, E, S, W); else move(num[0]);
}
// sidestep off the line d; both sides walls (a corridor): back off along it, sidestep next beat
function dodge(d) {
  if (d % 2 == 0 ? wall(x - 1, y) && wall(x + 1, y) : wall(x, y - 1) && wall(x, y + 1)) {
    num[2] = 2;
    move(back(d));
  } else { num[2] = 0; move(pick(left(d), right(d))); }
}
function blocked(d) {
  return wall(x + (d == 1 ? 1 : d == 3 ? -1 : 0), y + (d == 2 ? 1 : d == 0 ? -1 : 0));
}

// Lighthouse: hunter with a mode switch - sweep and dodge, hunt the shooter for a few passes after a hit.
// num[0] = sweep direction, num[1] = passes left to hunt the shooter (-1 = sidestep pending), num[2] = shooter direction
function loop() {
  if (num[1] < 0) { num[1] = 0; move(pick(left(num[0]), right(num[0]))); return; }   // retreated last beat: sidestep now
  if (num[1] > 0) { num[1]--; look(num[2]); return; }
  num[0] = right(num[0]);
  look(num[0]);
}
function look(d) {
  scan(d);
  if (bullet == 3) dodge(d);
  else if (enemy == 3 && loaded) fire(d);
  else if (enemy == 2) move(pick(left(d), right(d)));
  else if (coin == 3) move(d);
}
// sidestep off the line d; both sides walls (a corridor): back off along it, sidestep next beat
function dodge(d) {
  if (d % 2 == 0 ? wall(x - 1, y) && wall(x + 1, y) : wall(x, y - 1) && wall(x, y + 1)) {
    num[0] = d; num[1] = -1;
    move(back(d));
  } else move(pick(left(d), right(d)));
}
function onHit(from) {
  num[1] = 0;                                 // drop a stale pending sidestep
  dodge(from);                           // off the line first
  num[2] = from;
  if (num[1] >= 0) num[1] = 4;
}
function onBump(from) {
  if (loaded && chance(60)) fire(from); else move(back(from));
}

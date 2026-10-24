// Magpie: sweeps for coins and walks to them, steers clear of other bots and of gunfire.
// num[0] = sweep direction, num[1] = mode (0 sweep, 1 chase, 2 side check), num[2] = coin direction,
// num[3] = steps since the last look at the coin (side check: 9 = already crossed over), num[4] = side I stepped to,
// num[5] = 1 + danger direction (0 = none), num[6] = beats the danger is remembered, num[7] = beat in the sweep cycle;
// pin[0] = wander spot, pin[1] = the last sound I reacted to
function loop() {
  if (num[6] > 0) num[6]--; else num[5] = 0;
  if (heard != pin[1]) {                          // a new shot: get away from it
    pin[1] = heard;
    if (steps(pin[1]) >= 1 && steps(pin[1]) <= 6) { flee(path(pin[1]), 0); return; }
  }
  if (num[1] == 1) {                              // chasing a coin
    if (num[5] == num[2] + 1) { num[1] = 0; return; }   // the coin is toward danger: drop it
    if (num[3] >= 2) { look(num[2]); return; }    // re-check it every 2 steps
    num[3]++;
    if (blocked(num[2])) move(pick(left(num[2]), right(num[2]))); else move(num[2]);
    return;
  }
  if (num[1] == 2) { look(num[2]); return; }     // stepped off a lane: is the coin on my line now?
  num[7]++;
  if (num[7] > 6) num[7] = 1;
  if (num[7] <= 4) { num[0] = right(num[0]); look(num[0]); return; }   // 4 looks around
  wander();                                       // then 2 steps toward a new spot
  move(path(pin[0]));
}
function look(d) {
  scan(d);
  if (bullet == 3) { flee(d, 1); return; }
  if (enemy == 3 && loaded && hp >= 3) { fire(d); return; }   // already on its line and healthy: shoot first
  if (enemy >= 2) { flee(d, 0); return; }
  if (num[5] == d + 1 || coin == 0) { num[1] = 0; return; }   // nothing here, or it is toward danger
  if (coin == 3) { num[1] = 1; num[2] = d; num[3] = 1; move(d); return; }
  if (num[1] == 2) {                              // after a sidestep the coin is still off my line
    if (num[3] == 9) { num[1] = 1; num[2] = d; num[3] = 0; return; }   // crossed already: just head that way
    num[3] = 9;
    num[4] = back(num[4]);
    move(num[4]);                                 // wrong side: cross over to the other lane
    move(num[4]);
    return;
  }
  if (coin == 2) { num[1] = 2; num[2] = d; num[3] = 0; num[4] = pick(left(d), right(d)); move(num[4]); return; }
  num[1] = 1; num[2] = d; num[3] = 0;             // somewhere in the cone: walk that way, look again soon
}
// get off the threat's line d (side = 1: sidestep only, a bullet is coming), forget the coin, remember the danger
function flee(d, side) {
  num[1] = 0; num[7] = 0; num[5] = d + 1; num[6] = 5;
  pin[0] = none;
  if (side == 1) move(pick(left(d), right(d)));
  else move(pick(left(d), right(d), back(d), back(d)));
}
// keep the wander spot, or pick a reachable one that is not toward the danger
function wander() {
  if (pin[0] != none && path(pin[0]) >= 0) return;
  num[3] = 0;
  pin[0] = at(rand(1, WIDTH - 2), rand(1, HEIGHT - 2));
  while (num[3] < 6 && (path(pin[0]) < 0 || path(pin[0]) + 1 == num[5])) {
    num[3]++;
    pin[0] = at(rand(1, WIDTH - 2), rand(1, HEIGHT - 2));
  }
}
function blocked(d) {
  return wall(x + (d == 1 ? 1 : d == 3 ? -1 : 0), y + (d == 2 ? 1 : d == 0 ? -1 : 0));
}
function onHit(from) {
  flee(from, 1);
  num[6] = 7;
}
function onBump(from) {
  if (loaded) fire(from); else flee(from, 0);
}

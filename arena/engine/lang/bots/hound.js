// Hound: follows the sound of gunfire, looks before it steps, fires at a clear line. Recipe: hunter by ear - follow `heard`, look before each step, fire on 3, pin the last sighting.
// num[0] = direction of the next look, num[1] = looks done since the last sound I followed,
// num[2] = 1 when the next beat is a look toward the sound, num[3] = beats left to follow it;
// pin[0] = where I last saw an enemy, pin[1] = the sound I am following
function loop() {
  if (heard != pin[1]) { pin[1] = heard; num[3] = 3; }   // a new sound: follow it for a few beats (a repeat from the same cell looks the same)
  if (num[3] > 0 && path(pin[1]) >= 0) {      // creep toward it, a look between steps
    num[3]--;
    num[1] = 0;
    num[2] = 1 - num[2];
    if (num[2] == 1) { aim(path(pin[1])); return; }
    move(path(pin[1]));
    return;
  }
  if (num[1] < 4) { num[1]++; num[0] = right(num[0]); aim(num[0]); return; }   // arrived: look all around
  if (pin[0] != none && path(pin[0]) >= 0) { move(path(pin[0])); return; }     // back to the last sighting
  pin[0] = none;
  move(pick(N, E, S, W));                     // nothing heard: wander
}
function aim(d) {
  scan(d);
  if (enemy == 3) {
    pin[0] = here();                          // remember where I saw one
    if (loaded) fire(d);
  }
}

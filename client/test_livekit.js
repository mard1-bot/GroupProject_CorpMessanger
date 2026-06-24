const { Room, RoomEvent } = require('livekit-client');

async function test() {
  const roomName = "372d7da4-a85b-4069-ae64-94c0aadecfe2";
  const url = "wss://finchgram.ru/livekit/";
  const token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJleHAiOjE3ODIzMjgyNzUsImlzcyI6IlZnZS9kQXRWalI1ZmlxSjEiLCJtZXRhZGF0YSI6IntcIm5hbWVcIjogXCIzZjY4NzZiYS00NmU5LTRiYTYtOGUyNS1mN2ZmMmEwOGYwMWZcIn0iLCJuYmYiOjE3ODIzMDY2NzUsInN1YiI6IjNmNjg3NmJhLTQ2ZTktNGJhNi04ZTI1LWY3ZmYyYTA4ZjAxZiIsInZpZGVvIjp7InJvb20iOiIzNzJkN2RhNC1hODViLTQwNjktYWU2NC05NGMwYWFkZWNmZTIiLCJyb29tSm9pbiI6dHJ1ZX19.jj2-ecm0aYHnyqlGSGedGEmHH7TIbLXO8rJUMim8Udg";

  const room = new Room({ adaptiveStream: true, dynacast: true });
  
  room.on(RoomEvent.Connected, () => {
    console.log("Connected to room!");
    room.disconnect();
  });
  
  room.on(RoomEvent.Disconnected, () => {
    console.log("Disconnected from room");
  });

  try {
    await room.connect(url, token);
    console.log("Connect awaited successfully");
  } catch (e) {
    console.error("Connection failed:", e);
  }
}

test();

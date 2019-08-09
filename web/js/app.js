
function showError(error) {
  const errorNode = document.querySelector('#error');
  if (errorNode.firstChild) {
    errorNode.removeChild(errorNode.firstChild);
  }
  errorNode.appendChild(document.createTextNode(error.message || error));
}

function loadScreens() {
  return fetch('/api/screens', {
    method: 'GET',
    headers: {
      'Accepts': 'application/json'
    }
  }).then(res => {
    return res.json();
  }).catch(showError);
}

function startSession(offer, screen) {
  return fetch('/api/session', {
    method: 'POST',
    body: JSON.stringify({
      offer,
      screen
    }),
    headers: {
      'Content-Type': 'application/json'
    }
  }).then(res => {
    return res.json();
  }).then(msg => {
    return msg.answer;
  });
}

function createOffer(pc, { audio, video }) {
  return new Promise((accept, reject) => {
    pc.onicecandidate = evt => {
      if (!evt.candidate) {
        
        // ICE Gathering finished 
        const { sdp: offer } = pc.localDescription;
        accept(offer);
      }
    };
    pc.createOffer({
      offerToReceiveAudio: audio,
      offerToReceiveVideo: video
    }).then(ld => {
      pc.setLocalDescription(ld)
    }).catch(reject)
  });
}

function startRemoteSession(screen, remoteVideoNode, stream) {
  let pc;

  return Promise.resolve().then(() => {
    pc = new RTCPeerConnection({
      iceServers: [{ urls: 'stun:stun.l.google.com:19302' }]
    });
    pc.ontrack = (evt) => {
      console.info('ontrack triggered');
      
      remoteVideoNode.srcObject = evt.streams[0];
      remoteVideoNode.play();
    };

    stream && stream.getTracks().forEach(track => {
      pc.addTrack(track, stream);
    })
    return createOffer(pc, { audio: false, video: true });
  }).then(offer => {
    console.info(offer);
    return startSession(offer, screen);
  }).then(answer => {
    console.info(answer);
    return pc.setRemoteDescription(new RTCSessionDescription({
      sdp: answer,
      type: 'answer'
    }));
  }).then(() => pc);
}

let peerConnection = null;
document.addEventListener('DOMContentLoaded', () => {
  
  let selectedScreen = 0;
  const remoteVideo = document.querySelector('#remote-video');
  const screenSelect = document.querySelector('#screen-select');
  const startStop = document.querySelector('#start-stop');
  
  loadScreens().then(response => {
    while (screenSelect.firstChild) {
      screenSelect.removeChild(screenSelect.firstChild);
    }
    screenSelect.appendChild(document.createElement('option'));
    response.screens.forEach(screen => {
      const option = document.createElement('option');
      option.appendChild(document.createTextNode('Screen ' + (screen.index + 1)));
      option.setAttribute('value', screen.index);
      screenSelect.appendChild(option);
    });
  }).catch(showError);

  screenSelect.addEventListener('change', evt => {
    selectedScreen = parseInt(evt.currentTarget.value, 10);
  });

  const enableStartStop = (enabled) => {
    if (enabled) {
      startStop.removeAttribute('disabled');
    } else {
      startStop.setAttribute('disabled', '');
    }
  }

  const setStartStopTitle = (title) => {
    startStop.removeChild(startStop.firstChild);
    startStop.appendChild(document.createTextNode(title));
  }

  startStop.addEventListener('click', () => {
    enableStartStop(false);

    const userMediaPromise =  (adapter.browserDetails.browser === 'safari') ?
      navigator.mediaDevices.getUserMedia({ video: true }) : 
      Promise.resolve(null);
    if (!peerConnection) {
      userMediaPromise.then(stream => {
        return startRemoteSession(selectedScreen, remoteVideo, stream).then(pc => {
          remoteVideo.style.setProperty('visibility', 'visible');
          peerConnection = pc;
        }).catch(showError).then(() => {
          enableStartStop(true);
          setStartStopTitle('Stop');
        });
      })
    } else {
      peerConnection.close();
      peerConnection = null;
      enableStartStop(true);
      setStartStopTitle('Start');
      remoteVideo.style.setProperty('visibility', 'collapse');
    }
  });
});

window.addEventListener('beforeunload', () => {
  if (peerConnection) {
    peerConnection.close();
  }
})

package dev.localwhisper

import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.Service
import android.content.Intent
import android.content.pm.ServiceInfo
import android.media.AudioFormat
import android.media.AudioRecord
import android.media.MediaRecorder
import android.os.Build
import android.os.Handler
import android.os.IBinder
import android.util.Log
import androidx.core.app.NotificationCompat
import okhttp3.*
import okio.ByteString
import java.util.concurrent.TimeUnit

class DictationService : Service() {
    companion object {
        const val CONNECT = "dev.localwhisper.CONNECT"
        const val STATUS = "dev.localwhisper.STATUS"
        const val EXTRA_STATUS = "status"
        private const val CHANNEL = "dictation"
        private const val ID = 7
    }
    private val client = OkHttpClient.Builder()
        .connectTimeout(8, TimeUnit.SECONDS)
        .pingInterval(20, TimeUnit.SECONDS)
        .build()
    private val reconnect = Handler()
    private var socket: WebSocket? = null
    @Volatile private var recording = false
    private var recorder: AudioRecord? = null
    private var recordingThread: Thread? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        try {
            startForegroundNotification()
            connect()
        } catch (exception: SecurityException) {
            publishStatus("Connection: Disconnected\nMic: ${exception.message}")
            stopSelf(startId)
        } catch (exception: IllegalArgumentException) {
            publishStatus("Connection: Disconnected\nLaptop IP is invalid")
            stopSelf(startId)
        }
        return START_NOT_STICKY
    }
    override fun onBind(intent: Intent?): IBinder? = null
    override fun onDestroy() { stopRecording(); socket?.close(1000, "service stopped"); client.dispatcher.executorService.shutdown(); super.onDestroy() }

    private fun startForegroundNotification() {
        val manager = getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(NotificationChannel(CHANNEL, "LocalWhisper dictation", NotificationManager.IMPORTANCE_LOW))
        val notification = NotificationCompat.Builder(this, CHANNEL).setSmallIcon(android.R.drawable.ic_btn_speak_now).setContentTitle("LocalWhisper").setContentText("Connected microphone ready").build()
        if (Build.VERSION.SDK_INT >= 29) startForeground(ID, notification, ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE) else startForeground(ID, notification)
    }

    private fun connect() {
        if (socket != null) return
        val prefs = getSharedPreferences("localwhisper", MODE_PRIVATE)
        val host = prefs.getString("host", "") ?: return
        val token = prefs.getString("token", "") ?: ""
        val url = HttpUrl.Builder().scheme("http").host(host).port(8765).addPathSegment("ws").apply { if (token.isNotEmpty()) addQueryParameter("token", token) }.build()
        socket = client.newWebSocket(Request.Builder().url(url).build(), listener)
    }

    private val listener = object : WebSocketListener() {
        override fun onOpen(webSocket: WebSocket, response: Response) {
            webSocket.send("{\"type\":\"hello\",\"device\":\"${Build.MODEL.jsonEscape()}\"}")
            publishStatus("Connection: Connected\nMic: Idle")
        }
        override fun onMessage(webSocket: WebSocket, text: String) { when (Regex("\"type\"\\s*:\\s*\"([^\"]+)\"").find(text)?.groupValues?.get(1)) { "start" -> startRecording(); "stop" -> stopRecording() } }
        override fun onFailure(webSocket: WebSocket, t: Throwable, response: Response?) {
            socket = null
            val reason = t.message ?: t.javaClass.simpleName
            publishStatus("Connection: $reason\nMic: Idle")
            reconnect.postDelayed({ connect() }, 3000)
        }
        override fun onClosed(webSocket: WebSocket, code: Int, reason: String) {
            socket = null
            publishStatus("Connection: Disconnected\nMic: Idle")
            reconnect.postDelayed({ connect() }, 3000)
        }
    }

    private fun startRecording() {
        if (recording) return
        val minSize = AudioRecord.getMinBufferSize(16000, AudioFormat.CHANNEL_IN_MONO, AudioFormat.ENCODING_PCM_16BIT)
        if (minSize <= 0) return
        recorder = AudioRecord.Builder().setAudioSource(MediaRecorder.AudioSource.VOICE_RECOGNITION).setAudioFormat(AudioFormat.Builder().setSampleRate(16000).setChannelMask(AudioFormat.CHANNEL_IN_MONO).setEncoding(AudioFormat.ENCODING_PCM_16BIT).build()).setBufferSizeInBytes(minSize * 2).build()
        recording = true; recorder!!.startRecording()
        publishStatus("Connection: Connected\nMic: Recording")
        recordingThread = Thread {
            val buffer = ByteArray(minSize)
            while (recording) { val count = recorder?.read(buffer, 0, buffer.size) ?: -1; if (count > 0) socket?.send(ByteString.of(*buffer.copyOf(count))) }
        }.also { it.start() }
    }

    private fun stopRecording() {
        if (!recording) return
        recording = false; recorder?.stop(); recordingThread?.join(1000); recorder?.release(); recorder = null; recordingThread = null
        socket?.send("{\"type\":\"stopped\"}")
        publishStatus("Connection: Connected\nMic: Idle")
    }

    private fun publishStatus(value: String) {
        Log.i("LocalWhisper", "status: $value")
        sendBroadcast(Intent(STATUS).setPackage(packageName).putExtra(EXTRA_STATUS, value))
    }
    private fun String.jsonEscape() = replace("\\", "\\\\").replace("\"", "\\\"")
}

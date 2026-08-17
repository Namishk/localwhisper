package dev.localwhisper

import android.Manifest
import android.content.BroadcastReceiver
import android.content.Intent
import android.content.IntentFilter
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.util.Log
import android.view.Gravity
import android.widget.Button
import android.widget.EditText
import android.widget.LinearLayout
import android.widget.TextView
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AppCompatActivity
import androidx.core.content.ContextCompat

class MainActivity : AppCompatActivity() {
    private lateinit var status: TextView
    private lateinit var host: EditText
    private lateinit var token: EditText

    private val requestPermissions = registerForActivityResult(ActivityResultContracts.RequestMultiplePermissions()) { permissions ->
        if (permissions[Manifest.permission.RECORD_AUDIO] == true) {
            startService()
        } else {
            status.text = "Connection: Disconnected\nMic: permission required"
        }
    }

    private val statusReceiver = object : BroadcastReceiver() {
        override fun onReceive(context: android.content.Context, intent: Intent) {
            val value = intent.getStringExtra(DictationService.EXTRA_STATUS) ?: return
            Log.i("LocalWhisper", "status: $value")
            status.text = value
        }
    }

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val preferences = getSharedPreferences("localwhisper", MODE_PRIVATE)
        host = EditText(this).apply { hint = "Laptop IP (for example 192.168.1.20)"; setText(preferences.getString("host", "")) }
        token = EditText(this).apply { hint = "Pairing token (optional)"; setText(preferences.getString("token", "")) }
        status = TextView(this).apply { text = "Connection: Disconnected\nMic: Idle" }
        val connect = Button(this).apply { text = "Connect"; setOnClickListener { saveAndConnect() } }
        val layout = LinearLayout(this).apply {
            orientation = LinearLayout.VERTICAL; gravity = Gravity.CENTER_HORIZONTAL
            val padding = (24 * resources.displayMetrics.density).toInt(); setPadding(padding, padding, padding, padding)
            addView(TextView(this@MainActivity).apply { text = "LocalWhisper"; textSize = 28f })
            addView(host); addView(TextView(this@MainActivity).apply { text = "WebSocket port: 8765" })
            addView(token); addView(status); addView(connect)
        }
        setContentView(layout)
    }

    private fun saveAndConnect() {
        val laptop = host.text.toString().trim()
        if (laptop.isEmpty()) { host.error = "Enter the laptop LAN IP"; return }
        getSharedPreferences("localwhisper", MODE_PRIVATE).edit().putString("host", laptop).putString("token", token.text.toString().trim()).apply()
        val required = buildList { add(Manifest.permission.RECORD_AUDIO); if (Build.VERSION.SDK_INT >= 33) add(Manifest.permission.POST_NOTIFICATIONS) }
        if (required.any { ContextCompat.checkSelfPermission(this, it) != PackageManager.PERMISSION_GRANTED }) { requestPermissions.launch(required.toTypedArray()) } else { startService() }
    }

    private fun startService() {
        status.text = "Connection: Connecting…\nMic: Idle"
        try {
            ContextCompat.startForegroundService(this, Intent(this, DictationService::class.java).setAction(DictationService.CONNECT))
        } catch (exception: SecurityException) {
            status.text = "Connection: Disconnected\nMic: ${exception.message}"
            return
        } catch (exception: IllegalStateException) {
            status.text = "Connection: Disconnected\nService: ${exception.message}"
            return
        }
    }

    override fun onResume() {
        super.onResume()
        ContextCompat.registerReceiver(this, statusReceiver, IntentFilter(DictationService.STATUS), ContextCompat.RECEIVER_NOT_EXPORTED)
    }

    override fun onPause() {
        unregisterReceiver(statusReceiver)
        super.onPause()
    }
}

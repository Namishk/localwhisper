import org.jetbrains.kotlin.gradle.dsl.JvmTarget
import java.util.Properties

plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

val signingProperties = Properties()
val signingPropertiesFile = rootProject.file("signing.properties")
if (signingPropertiesFile.isFile) {
    signingPropertiesFile.inputStream().use(signingProperties::load)
}

android {
    namespace = "dev.localwhisper"
    compileSdk = 35
    buildToolsVersion = "35.0.1"

    compileOptions {
        sourceCompatibility = JavaVersion.VERSION_1_8
        targetCompatibility = JavaVersion.VERSION_1_8
    }
    defaultConfig {
        applicationId = "dev.localwhisper"
        minSdk = 26
        targetSdk = 35
        versionCode = 6
        versionName = "1.1.0"
    }

    signingConfigs {
        create("release") {
            val storeFilePath = signingProperties.getProperty("storeFile")
                ?: error("Create android/signing.properties from signing.properties.example before building a release APK")
            storeFile = rootProject.file(storeFilePath)
            storePassword = signingProperties.getProperty("storePassword")
            keyAlias = signingProperties.getProperty("keyAlias")
            keyPassword = signingProperties.getProperty("keyPassword")
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.getByName("release")
        }
    }
}

kotlin { compilerOptions { jvmTarget.set(JvmTarget.JVM_1_8) } }

dependencies {
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.activity:activity-ktx:1.10.0")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
}

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
        versionCode = 7
        versionName = "1.3.0"
    }

    // The release signing config only exists when signing.properties is present,
    // so debug builds and CI work from a clean checkout. Release builds without
    // it fail in the guard below this block.
    signingConfigs {
        if (signingPropertiesFile.isFile) {
            create("release") {
                storeFile = rootProject.file(signingProperties.getProperty("storeFile"))
                storePassword = signingProperties.getProperty("storePassword")
                keyAlias = signingProperties.getProperty("keyAlias")
                keyPassword = signingProperties.getProperty("keyPassword")
            }
        }
    }

    buildTypes {
        release {
            isMinifyEnabled = false
            signingConfig = signingConfigs.findByName("release")
        }
    }
}

// Fail a release build clearly rather than emitting an unsigned APK. Debug
// builds and CI never need signing.properties.
if (!signingPropertiesFile.isFile &&
    gradle.startParameter.taskNames.any { it.contains("Release", ignoreCase = false) }
) {
    error("Create android/signing.properties from signing.properties.example before building a release APK")
}

kotlin { compilerOptions { jvmTarget.set(JvmTarget.JVM_1_8) } }

dependencies {
    implementation("androidx.core:core-ktx:1.15.0")
    implementation("androidx.activity:activity-ktx:1.10.0")
    implementation("androidx.appcompat:appcompat:1.7.0")
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
}

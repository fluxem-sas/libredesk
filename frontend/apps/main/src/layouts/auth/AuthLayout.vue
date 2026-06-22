<template>
  <div class="auth-layout animate-auth-fade-in">
    <AuthPreferences />

    <div class="auth-layout__visual">
      <div class="auth-layout__visual-overlay"></div>
      <div class="auth-layout__visual-logo">
        <img
          :src="logoUrl"
          alt="Fluxem"
          class="auth-layout__visual-logo-image"
        />
      </div>
      <img
        :src="isotypeUrl"
        alt="FluxemDesk"
        class="auth-layout__visual-image"
      />
    </div>

    <div class="auth-layout__content">
      <div class="auth-layout__container">
        <slot></slot>
      </div>
    </div>
  </div>
</template>

<script setup>
import isotypeUrl from '/images/login-isotype.svg?url'
import logoUrl from '/images/logo-fluxem.svg?url'
import AuthPreferences from '@/components/auth/AuthPreferences.vue'
</script>
